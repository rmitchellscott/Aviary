import { useState, useEffect, useMemo } from "react";
import { useAuth } from "@/components/AuthProvider";
import { LoginForm } from "@/components/LoginForm";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogClose,
} from "@/components/ui/dialog";
import { FileDropzone } from "@/components/FileDropzone";
import { Loader2, CircleCheck, XCircle } from "lucide-react";
import { Progress } from "@/components/ui/progress";
import { useTranslation } from "react-i18next";
import i18n from "@/lib/i18n";

// List of file extensions (lower-cased, including the dot) that we allow compression for:
const COMPRESSIBLE_EXTS = [".pdf", ".png", ".jpg", ".jpeg"];
// How often to animate progress bar updates (ms)
const POLL_INTERVAL_MS = 200;

// Job status from backend
interface JobStatus {
  status: string;
  message: string;
  progress: number;
  operation?: string;
}

function waitForJobWS(
  jobId: string,
  onUpdate: (st: JobStatus) => void,
): Promise<void> {
  return new Promise((resolve) => {
    const proto = window.location.protocol === "https:" ? "wss:" : "ws:";
    const ws = new WebSocket(
      `${proto}//${window.location.host}/api/status/ws/${jobId}`,
    );
    ws.onmessage = (ev) => {
      try {
        const st = JSON.parse(ev.data);
        onUpdate(st);
        if (st.status === "success" || st.status === "error") {
          ws.close();
        }
      } catch {
        // ignore parse errors
      }
    };
    ws.onclose = () => resolve();
    ws.onerror = () => resolve();
  });
}

/**
 * Helper to turn any thrown value into a string.
 */
function getErrorMessage(err: unknown): string {
  if (err instanceof Error) {
    return err.message;
  }
  return String(err);
}

async function sniffMime(url: string): Promise<string | null> {
  try {
    const resp = await fetch(`/api/sniff?url=${encodeURIComponent(url)}`, {
      headers: {
        "Accept-Language": i18n.language
      }
    });
    if (!resp.ok) return null;
    const data = await resp.json();
    return data.mime || null;
  } catch {
    return null;
  }
}

export default function HomePage() {
  const { isAuthenticated, isLoading, login, authConfigured, uiSecret } =
    useAuth();
  const { t } = useTranslation();
  const [url, setUrl] = useState<string>("");
  const [committedUrl, setCommittedUrl] = useState<string>("");
  const [urlMime, setUrlMime] = useState<string | null>(null);
  const [selectedFile, setSelectedFile] = useState<File | null>(null);
  const [compress, setCompress] = useState<boolean>(false);
  const [loading, setLoading] = useState<boolean>(false);
  const [status, setStatus] = useState<string>("");
  const [message, setMessage] = useState<string>("");
  const [progress, setProgress] = useState<number>(0);
  const [operation, setOperation] = useState<string>("");
  const [fileError, setFileError] = useState<string | null>(null);
  const DEFAULT_RM_DIR = "default";
  const [folders, setFolders] = useState<string[]>([]);
  const [foldersLoading, setFoldersLoading] = useState<boolean>(true);
  const [rmDir, setRmDir] = useState<string>(DEFAULT_RM_DIR);

  /**
   * Determine if "Compress PDF" should be enabled:
   * - File mode: only if selected file name ends with a compressible extension
   * - URL mode: if URL ends with a compressible extension; if URL has some other extension, disable;
   *   if no extension, keep enabled.
   */
  const isCompressibleFileOrUrl = useMemo(() => {
    if (selectedFile) {
      // See if selectedFile.name ends with any compressible extension
      const lowerName = selectedFile.name.toLowerCase();
      return COMPRESSIBLE_EXTS.some((ext) => lowerName.endsWith(ext));
    }

    const trimmed = committedUrl.trim().toLowerCase();
    if (!trimmed) {
      // No URL entered → allow compress switch (harmless if clicked before submit)
      return true;
    }

    // Does URL end with a compressible extension?
    if (COMPRESSIBLE_EXTS.some((ext) => trimmed.endsWith(ext))) {
      return true;
    }

    // Check for any other extension in the last path segment
    const lastSegment = trimmed.split("/").pop() || "";
    if (lastSegment.includes(".")) {
      // If it has a dot but doesn't end with a compressible one, disable
      return false;
    }

    // No extension in URL (e.g. "https://example.com/download")
    if (urlMime) {
      const mt = urlMime.toLowerCase();
      return (
        mt.startsWith("application/pdf") ||
        mt.startsWith("image/png") ||
        mt.startsWith("image/jpeg")
      );
    }
    // No MIME info → allow compress
    return true;
  }, [selectedFile, committedUrl, urlMime]);

  useEffect(() => {
    if (!isCompressibleFileOrUrl && compress) {
      setCompress(false);
    }
  }, [isCompressibleFileOrUrl, compress]);

  useEffect(() => {
    if (!isAuthenticated) return;
    (async () => {
      try {
        const headers: HeadersInit = {
          "Accept-Language": i18n.language
        };
        if (uiSecret) {
          headers["X-UI-Token"] = uiSecret;
        }
        const res = await fetch("/api/folders", {
          headers,
          credentials: "include",
        }).then((r) => r.json());

        if (Array.isArray(res.folders)) {
          const cleaned = res.folders
            .map((f: string) => f.replace(/^\//, ""))
            .filter((f: string) => f !== "");
          setFolders(cleaned);
        }

        // Fetch an up-to-date list in the background and update state when done
        const refreshHeaders: HeadersInit = {
          "Accept-Language": i18n.language
        };
        if (uiSecret) {
          refreshHeaders["X-UI-Token"] = uiSecret;
        }
        fetch("/api/folders?refresh=1", {
          headers: refreshHeaders,
          credentials: "include",
        })
          .then((r) => r.json())
          .then((fresh) => {
            if (Array.isArray(fresh.folders)) {
              const cleanedFresh = fresh.folders
                .map((f: string) => f.replace(/^\//, ""))
                .filter((f: string) => f !== "");
              setFolders(cleanedFresh);
            }
          })
          .catch((err) => console.error("Failed to refresh folders:", err));
      } catch (error) {
        console.error("Failed to fetch folders:", error);
      }
      setFoldersLoading(false);
    })();
  }, [isAuthenticated]);

  if (isLoading) {
    return null;
  }

  // Show login form if auth is configured and not authenticated
  if (authConfigured && !isAuthenticated) {
    return <LoginForm onLogin={login} />;
  }

  /**
   * If a local file is selected, POST it to /api/upload as multipart/form-data.
   * Otherwise, enqueue by sending a URL to /api/webhook (old behavior).
   */
  const handleSubmit = async () => {
    setLoading(true);
    setMessage("");
    setStatus("");
    setOperation("");

    if (selectedFile) {
      // === FILE UPLOAD FLOW (enqueue + poll) ===
      try {
        const formData = new FormData();
        formData.append("file", selectedFile);
        formData.append("compress", compress ? "true" : "false");
        if (rmDir !== DEFAULT_RM_DIR) {
          formData.append("rm_dir", rmDir);
        }

        // 1) send to /api/upload and get back { jobId }
        const headers: HeadersInit = {};
        if (uiSecret) {
          headers["X-UI-Token"] = uiSecret;
        }
        // Include current language for backend i18n
        headers["Accept-Language"] = i18n.language;
        
        const res = await fetch("/api/upload", {
          method: "POST",
          headers,
          credentials: "include",
          body: formData,
        });
        if (!res.ok) {
          const errText = await res.text();
          throw new Error(`Upload failed: ${errText}`);
        }
        const { jobId } = await res.json();
        setMessage(`Job queued: ${jobId}`);

        setStatus("running");
        setProgress(0);
        await waitForJobWS(jobId, (st) => {
          setStatus(st.status.toLowerCase());
          setMessage(st.message);
          setOperation(st.operation || "");
          if (typeof st.progress === "number") {
            setProgress(st.progress);
          }
        });
      } catch (err: unknown) {
        const msg = getErrorMessage(err);
        setStatus("error");
        setMessage(msg);
      } finally {
        // clear the selected file so <Input> becomes enabled again
        setSelectedFile(null);
        setUrl("");
        setProgress(0);
        setOperation("");
        setLoading(false);
      }
    } else {
      // === URL SUBMIT FLOW (EXISTING) ===
      const form = new URLSearchParams();
      form.append("Body", url);
      form.append("compress", compress ? "true" : "false");
      if (rmDir !== DEFAULT_RM_DIR) {
        form.append("rm_dir", rmDir);
      }

      try {
        const headers: HeadersInit = { 
          "Content-Type": "application/x-www-form-urlencoded",
          "Accept-Language": i18n.language
        };
        if (uiSecret) {
          headers["X-UI-Token"] = uiSecret;
        }
        
        const res = await fetch("/api/webhook", {
          method: "POST",
          headers,
          credentials: "include",
          body: form.toString(),
        });
        if (!res.ok) {
          const errText = await res.text();
          throw new Error(`Enqueue failed: ${errText}`);
        }
        const { jobId } = await res.json();
        setStatus("running");
        setMessage(`Job queued: ${jobId}`);
        setProgress(0);

        await waitForJobWS(jobId, (st) => {
          setStatus(st.status.toLowerCase());
          setMessage(st.message);
          setOperation(st.operation || "");
          if (typeof st.progress === "number") {
            setProgress(st.progress);
          }
        });
      } catch (err: unknown) {
        const msg = getErrorMessage(err);
        setStatus("error");
        setMessage(msg);
      } finally {
        setUrl("");
        setProgress(0);
        setOperation("");
        setLoading(false);
      }
    }
  };

  return (
    <div className="bg-background pt-0 pb-8 px-8">
      <Card className="max-w-md mx-auto bg-card">
        <CardHeader>
          <CardTitle className="text-xl">{t('home.send_document')}</CardTitle>
        </CardHeader>

        <CardContent className="space-y-6">
          {/* === URL INPUT === */}
          <div>
            <Input
              id="url"
              type="text"
              value={url}
              onChange={(e: React.ChangeEvent<HTMLInputElement>) => {
                setUrl(e.target.value);
                // Clear any selected file if the user starts typing a URL
                if (selectedFile) {
                  setSelectedFile(null);
                }
                setUrlMime(null);
              }}
              onBlur={async () => {
                // commit the URL once the user leaves the field
                setCommittedUrl(url);
                if (url.trim()) {
                  const mt = await sniffMime(url.trim());
                  setUrlMime(mt);
                } else {
                  setUrlMime(null);
                }
              }}
              placeholder={t('home.url_placeholder')}
              disabled={!!selectedFile}
            />
          </div>

          <div className="text-center text-sm text-muted-foreground">
            {t('home.or')}
          </div>

          {/* === DRAG & DROP FILE === */}
          <div>
            <FileDropzone
              onFileSelected={(file) => {
                setSelectedFile(file);
                // Clear any URL if the user picks a file
                if (url) {
                  setUrl("");
                }
                setUrlMime(null);
              }}
              onError={(msg) => {
                // Set the error text—this will open the Dialog
                setFileError(msg);
              }}
              disabled={!!url}
            />
            {selectedFile && (
              <div className="mt-2 flex justify-between items-center">
                <p className="text-sm text-foreground">
                  {t('home.selected_file')} {" "}
                  <span className="font-medium">{selectedFile.name}</span>
                </p>
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={() => setSelectedFile(null)}
                >
                  {t('home.remove')}
                </Button>
              </div>
            )}
          </div>

          {/* === FOLDER SELECT === */}
          <div className="flex items-center space-x-2">
            <Label htmlFor="rmDir">{t('home.destination_folder')}</Label>
            <Select value={rmDir} onValueChange={setRmDir}>
              <SelectTrigger id="rmDir">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value={DEFAULT_RM_DIR}>{t('home.default')}</SelectItem>
                {foldersLoading && (
                  <SelectItem value="loading" disabled>
                    {t('home.loading')}
                  </SelectItem>
                )}
                {folders.map((f) => (
                  <SelectItem key={f} value={f}>
                    {f}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          {/* === COMPRESS SWITCH === */}
          <div className="flex items-center space-x-2 mt-4">
            <Label
              htmlFor="compress"
              className={!isCompressibleFileOrUrl ? "opacity-50" : ""}
            >
              {t('home.compress_pdf')}
            </Label>
            <Switch
              id="compress"
              checked={compress}
              onCheckedChange={setCompress}
              disabled={!isCompressibleFileOrUrl}
            />
          </div>

          {/* === SUBMIT BUTTON === */}
          <div className="flex justify-end">
            <Button
              onClick={handleSubmit}
              disabled={loading || (!url && !selectedFile)}
            >
              {loading ? t('home.sending') : t('home.send')}
            </Button>
          </div>

          {message && (
            <div className="mt-2 flex items-center gap-2 rounded-md bg-secondary px-3 py-2 text-sm text-secondary-foreground">
              {status === "running" && (
                <Loader2 className="size-4 flex-shrink-0 animate-spin" />
              )}
              {status === "success" && (
                <CircleCheck className="size-4 flex-shrink-0 text-primary" />
              )}
              {status === "error" && (
                <XCircle className="size-4 flex-shrink-0 text-destructive" />
              )}
              <span className="break-words">{message}</span>
            </div>
          )}
          {status === "running" &&
            operation === "compressing" &&
            progress > 0 &&
            progress < 100 && (
              <Progress
                value={progress}
                durationMs={POLL_INTERVAL_MS}
                className="mt-2"
              />
            )}
        </CardContent>
      </Card>

      {/* === ERROR DIALOG === */}
      <Dialog
        open={!!fileError}
        onOpenChange={(open) => {
          if (!open) setFileError(null);
        }}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('home.invalid_file')}</DialogTitle>
            <DialogDescription>{fileError}</DialogDescription>
          </DialogHeader>
          <div className="mt-4 flex justify-end">
            <DialogClose asChild>
              <Button>{t('home.ok')}</Button>
            </DialogClose>
          </div>
        </DialogContent>
      </Dialog>
    </div>
  );
}

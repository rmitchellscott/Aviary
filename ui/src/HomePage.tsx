import { useState, useEffect, useMemo } from "react";
import { useAuth } from "@/components/AuthProvider";
import { LoginForm } from "@/components/LoginForm";
import { PairingDialog } from "@/components/PairingDialog";
import { useUserData } from "@/hooks/useUserData";
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

const COMPRESSIBLE_EXTS = [".pdf", ".png", ".jpg", ".jpeg"];
const POLL_INTERVAL_MS = 200;

interface JobStatus {
  status: string;
  message: string;
  data?: Record<string, string>;
  progress: number;
  operation?: string;
}

function waitForJobWS(
  jobId: string,
  onUpdate: (st: JobStatus) => void,
): Promise<void> {
  return new Promise((resolve) => {
    let resolved = false;
    const safeResolve = () => {
      if (!resolved) {
        resolved = true;
        resolve();
      }
    };

    const proto = window.location.protocol === "https:" ? "wss:" : "ws:";
    const ws = new WebSocket(
      `${proto}//${window.location.host}/api/status/ws/${jobId}`,
    );
    ws.onmessage = (ev) => {
      try {
        const st = JSON.parse(ev.data);
        onUpdate(st);
        if (st.status === "success" || st.status === "error") {
          setTimeout(() => {
            ws.close();
            safeResolve();
          }, 10);
          return;
        }
      } catch {
      }
    };
    ws.onclose = () => safeResolve();
    ws.onerror = () => safeResolve();
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
        "Accept-Language": i18n.language,
      },
    });
    if (!resp.ok) return null;
    const data = await resp.json();
    return data.mime || null;
  } catch {
    return null;
  }
}

export default function HomePage() {
  const { isAuthenticated, isLoading, login, authConfigured, uiSecret, multiUserMode, oidcEnabled, proxyAuthEnabled } =
    useAuth();
  const { t } = useTranslation();
  const { rmapiPaired, rmapiHost, loading: userDataLoading, updatePairingStatus } = useUserData();
  
  const [url, setUrl] = useState<string>("");
  const [committedUrl, setCommittedUrl] = useState<string>("");
  const [urlMime, setUrlMime] = useState<string | null>(null);
  const [selectedFile, setSelectedFile] = useState<File | null>(null);
  const [compress, setCompress] = useState<boolean>(false);
  const [loading, setLoading] = useState<boolean>(false);
  const [status, setStatus] = useState<string>("");
  const [message, setMessage] = useState<string>("");
  const [progress, setProgress] = useState<number>(0);
  const [uploadProgress, setUploadProgress] = useState<number>(0);
  const [uploadPhase, setUploadPhase] = useState<'idle' | 'uploading' | 'processing'>('idle');
  const [fileError, setFileError] = useState<string | null>(null);
  const DEFAULT_RM_DIR = "default";
  const [folders, setFolders] = useState<string[]>([]);
  const [foldersLoading, setFoldersLoading] = useState<boolean>(true);
  const [rmDir, setRmDir] = useState<string>(DEFAULT_RM_DIR);
  
  // Pairing dialog state
  const [pairingDialogOpen, setPairingDialogOpen] = useState(false);

  const isCompressibleFileOrUrl = useMemo(() => {
    if (selectedFile) {
      const lowerName = selectedFile.name.toLowerCase();
      return COMPRESSIBLE_EXTS.some((ext) => lowerName.endsWith(ext));
    }

    const trimmed = committedUrl.trim();
    if (!trimmed) {
      return true;
    }

    let path = trimmed;
    try {
      path = new URL(trimmed).pathname;
    } catch {
    }
    const lowerPath = path.toLowerCase();

    if (COMPRESSIBLE_EXTS.some((ext) => lowerPath.endsWith(ext))) {
      return true;
    }

    const lastSegment = lowerPath.split("/").pop() || "";
    if (lastSegment.includes(".")) {
      return false;
    }

    if (urlMime) {
      const mt = urlMime.toLowerCase();
      return (
        mt.startsWith("application/pdf") ||
        mt.startsWith("image/png") ||
        mt.startsWith("image/jpeg")
      );
    }
    return true;
  }, [selectedFile, committedUrl, urlMime]);

  useEffect(() => {
    if (!isCompressibleFileOrUrl && compress) {
      setCompress(false);
    }
  }, [isCompressibleFileOrUrl, compress]);

  useEffect(() => {
    if (!isAuthenticated || !rmapiPaired || userDataLoading) {
      setFolders([]);
      setFoldersLoading(false);
      return;
    }

    (async () => {
      try {
        setFoldersLoading(true);
        const headers: HeadersInit = {
          "Accept-Language": i18n.language,
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

        const refreshHeaders: HeadersInit = {
          "Accept-Language": i18n.language,
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
      } finally {
        setFoldersLoading(false);
      }
    })();
  }, [isAuthenticated, rmapiPaired, userDataLoading]);

  const handlePairingSuccess = () => {
    updatePairingStatus(true);
  };

  if (isLoading) {
    return null;
  }

  if (authConfigured && !isAuthenticated) {
    return <LoginForm onLogin={login} />;
  }

  const uploadFileWithProgress = async (formData: FormData): Promise<{ jobId: string }> => {
    return new Promise((resolve, reject) => {
      const xhr = new XMLHttpRequest();
      
      xhr.upload.addEventListener('progress', (event) => {
        if (event.lengthComputable) {
          const percentComplete = (event.loaded / event.total) * 100;
          setUploadProgress(percentComplete);
        }
      });
      
      xhr.addEventListener('load', () => {
        if (xhr.status >= 200 && xhr.status < 300) {
          try {
            const response = JSON.parse(xhr.responseText);
            resolve(response);
          } catch (err) {
            reject(new Error('Invalid response format'));
          }
        } else {
          reject(new Error(xhr.responseText || `HTTP ${xhr.status}`));
        }
      });
      
      xhr.addEventListener('error', () => {
        reject(new Error('Upload failed'));
      });
      
      xhr.open('POST', '/api/upload');
      
      if (uiSecret) {
        xhr.setRequestHeader('X-UI-Token', uiSecret);
      }
      xhr.setRequestHeader('Accept-Language', i18n.language);
      xhr.withCredentials = true;
      
      xhr.send(formData);
    });
  };
  const handleSubmit = async () => {
    setLoading(true);
    setMessage("");
    setStatus("");
    setUploadProgress(0);
    setUploadPhase('idle');

    if (selectedFile) {
      try {
        const formData = new FormData();
        formData.append("file", selectedFile);
        formData.append("compress", compress ? "true" : "false");
        if (rmDir !== DEFAULT_RM_DIR) {
          formData.append("rm_dir", rmDir);
        }

        setUploadPhase('uploading');
        setMessage(t("home.uploading"));
        const { jobId } = await uploadFileWithProgress(formData);
        
        setUploadPhase('processing');
        setMessage(t("home.job_queued", { id: jobId }));
        setUploadProgress(100); // Upload complete

        setStatus("running");
        setProgress(0);
        await waitForJobWS(jobId, (st) => {
          setStatus(st.status.toLowerCase());
          setMessage(t(st.message, st.data || {}));
          if (typeof st.progress === "number") {
            setProgress(st.progress);
          }
        });
      } catch (err: unknown) {
        const msg = getErrorMessage(err);
        setStatus("error");
        setMessage(t(msg));
      } finally {
        setSelectedFile(null);
        setUrl("");
        setProgress(0);
        setUploadProgress(0);
        setUploadPhase('idle');
        setLoading(false);
      }
    } else {
      const form = new URLSearchParams();
      form.append("Body", url);
      form.append("compress", compress ? "true" : "false");
      if (rmDir !== DEFAULT_RM_DIR) {
        form.append("rm_dir", rmDir);
      }

      try {
        const headers: HeadersInit = {
          "Content-Type": "application/x-www-form-urlencoded",
          "Accept-Language": i18n.language,
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
          throw new Error(errText);
        }
        const { jobId } = await res.json();
        setStatus("running");
        setMessage(t("home.job_queued", { id: jobId }));
        setProgress(0);

        await waitForJobWS(jobId, (st) => {
          setStatus(st.status.toLowerCase());
          setMessage(t(st.message, st.data || {}));
          if (typeof st.progress === "number") {
            setProgress(st.progress);
          }
        });
      } catch (err: unknown) {
        const msg = getErrorMessage(err);
        setStatus("error");
        setMessage(t(msg));
      } finally {
        setUrl("");
        setProgress(0);
        setLoading(false);
      }
    }
  };

  return (
    <div className="bg-background pt-0 pb-8 px-8">
      <Card className="max-w-md mx-auto bg-card">
        <CardHeader>
          <CardTitle className="text-xl">{t("home.send_document")}</CardTitle>
        </CardHeader>

        <CardContent className="space-y-6">
          <div>
            <Input
              id="url"
              type="text"
              value={url}
              onChange={(e: React.ChangeEvent<HTMLInputElement>) => {
                setUrl(e.target.value);
                if (selectedFile) {
                  setSelectedFile(null);
                }
                setUrlMime(null);
              }}
              onBlur={async () => {
                setCommittedUrl(url);
                if (url.trim()) {
                  const mt = await sniffMime(url.trim());
                  setUrlMime(mt);
                } else {
                  setUrlMime(null);
                }
              }}
              placeholder={t("home.url_placeholder")}
              disabled={!!selectedFile}
            />
          </div>

          <div className="text-center text-sm text-muted-foreground">
            {t("home.or")}
          </div>

          <div>
            <FileDropzone
              onFileSelected={(file) => {
                setSelectedFile(file);
                if (url) {
                  setUrl("");
                }
                setUrlMime(null);
              }}
              onError={(msg) => {
                setFileError(msg);
              }}
              disabled={!!url}
            />
            {selectedFile && (
              <div className="mt-2 flex justify-between items-center">
                <p className="text-sm text-foreground">
                  {t("home.selected_file")}{" "}
                  <span className="font-medium">{selectedFile.name}</span>
                </p>
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={() => setSelectedFile(null)}
                >
                  {t("home.remove")}
                </Button>
              </div>
            )}
          </div>

          <div className="flex items-center space-x-2">
            <Label htmlFor="rmDir">{t("home.destination_folder")}</Label>
            <Select 
              value={rmDir} 
              onValueChange={setRmDir}
              disabled={!rmapiPaired}
            >
              <SelectTrigger id="rmDir">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value={DEFAULT_RM_DIR}>
                  {t("home.default")}
                </SelectItem>
                {!rmapiPaired && (
                  <SelectItem value="not-paired" disabled>
                    {t("home.pair_with_cloud")}
                  </SelectItem>
                )}
                {rmapiPaired && foldersLoading && (
                  <SelectItem value="loading" disabled>
                    {t("home.loading")}
                  </SelectItem>
                )}
                {rmapiPaired && folders.map((f) => (
                  <SelectItem key={f} value={f}>
                    {f}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          <div className="flex items-center space-x-2 mt-4">
            <Label
              htmlFor="compress"
              className={!isCompressibleFileOrUrl ? "opacity-50" : ""}
            >
              {t("home.compress_pdf")}
            </Label>
            <Switch
              id="compress"
              checked={compress}
              onCheckedChange={setCompress}
              disabled={!isCompressibleFileOrUrl}
            />
          </div>

          {!rmapiPaired && !userDataLoading && (
            <div className="bg-muted border rounded-md p-3 text-muted-foreground">
              <p className="text-sm">
                <strong className="text-foreground">{t("home.pair_with_remarkable")}</strong>{t("home.to_upload_documents")}{multiUserMode && (
                  <>{t("home.settings_config")}</>
                )}
              </p>
            </div>
          )}

          <div className="flex justify-end">
            <Button
              onClick={!rmapiPaired ? () => setPairingDialogOpen(true) : handleSubmit}
              disabled={loading || (!url && !selectedFile && rmapiPaired)}
            >
              {loading ? t("home.sending") : !rmapiPaired ? t("home.pair") : t("home.send")}
            </Button>
          </div>

          {message && (
            <div className="mt-2 flex items-center gap-2 rounded-md bg-secondary px-3 py-2 text-sm text-secondary-foreground">
              {(status === "running" || uploadPhase === 'uploading') && (
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
          {(uploadPhase === 'uploading' && uploadProgress > 0 && uploadProgress < 100) ||
           (status === "running" && progress > 0 && progress < 100) ? (
              <Progress
                value={uploadPhase === 'uploading' ? uploadProgress : progress}
                durationMs={POLL_INTERVAL_MS}
                className="mt-2"
              />
            ) : null}
        </CardContent>
      </Card>

      <Dialog
        open={!!fileError}
        onOpenChange={(open) => {
          if (!open) setFileError(null);
        }}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("home.invalid_file")}</DialogTitle>
            <DialogDescription>{fileError}</DialogDescription>
          </DialogHeader>
          <div className="mt-4 flex justify-end">
            <DialogClose asChild>
              <Button>{t("home.ok")}</Button>
            </DialogClose>
          </div>
        </DialogContent>
      </Dialog>

      <PairingDialog
        isOpen={pairingDialogOpen}
        onClose={() => setPairingDialogOpen(false)}
        onPairingSuccess={handlePairingSuccess}
        rmapiHost={rmapiHost}
      />
    </div>
  );
}

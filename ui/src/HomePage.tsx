import { useState, useEffect, useMemo, useRef } from "react";
import { useAuth } from "@/components/AuthProvider";
import { LoginForm } from "@/components/LoginForm";
import { PairingDialog } from "@/components/PairingDialog";
import { useUserData } from "@/hooks/useUserData";
import { useConfig } from "@/components/ConfigProvider";
import { useFolderRefresh } from "@/hooks/useFolderRefresh";
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

const globalFoldersFetch = {
  isFetching: false,
  lastFetchTime: 0,
  abortController: null as AbortController | null,
  currentPromise: null as Promise<any> | null,
  requestCounter: 0,
};

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

// Helper function to truncate long directory paths from the beginning
const truncateFromStart = (path: string, maxLength: number = 40): string => {
  if (path.length <= maxLength) return path;
  return '...' + path.slice(-(maxLength - 3));
};

export default function HomePage() {
  const { isAuthenticated, isLoading, login, authConfigured, uiSecret, multiUserMode, oidcEnabled, proxyAuthEnabled } =
    useAuth();
  const { t } = useTranslation();
  const { rmapiPaired, rmapiHost, loading: userDataLoading, updatePairingStatus, user } = useUserData();
  const { config } = useConfig();
  const { refreshTrigger, triggerRefresh } = useFolderRefresh();
  const [url, setUrl] = useState<string>("");
  const [committedUrl, setCommittedUrl] = useState<string>("");
  const [urlMime, setUrlMime] = useState<string | null>(null);
  const [selectedFile, setSelectedFile] = useState<File | null>(null);
  const [selectedFiles, setSelectedFiles] = useState<File[]>([]);
  const [compress, setCompress] = useState<boolean>(false);
  const [removeBackground, setRemoveBackground] = useState<boolean>(false);
  const [loading, setLoading] = useState<boolean>(false);
  const [status, setStatus] = useState<string>("");
  const [message, setMessage] = useState<string>("");
  const [statusData, setStatusData] = useState<Record<string, string> | null>(null);
  const [progress, setProgress] = useState<number>(0);
  const [uploadProgress, setUploadProgress] = useState<number>(0);
  const [uploadPhase, setUploadPhase] = useState<'idle' | 'uploading' | 'processing'>('idle');
  const [fileError, setFileError] = useState<string | null>(null);
  const DEFAULT_RM_DIR = "default";
  const [folders, setFolders] = useState<string[]>([]);
  const [foldersLoading, setFoldersLoading] = useState<boolean>(false);
  const [rmDir, setRmDir] = useState<string>(DEFAULT_RM_DIR);
  
  const isFetchingFolders = useRef(false);
  const hasFetchedInitial = useRef(false);
  
  const [pairingDialogOpen, setPairingDialogOpen] = useState(false);

  const isCompressibleFileOrUrl = useMemo(() => {
    if (selectedFiles.length > 0) {
      return selectedFiles.some(file => {
        const lowerName = file.name.toLowerCase();
        return COMPRESSIBLE_EXTS.some((ext) => lowerName.endsWith(ext));
      });
    }
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
  }, [selectedFile, selectedFiles, committedUrl, urlMime]);

  useEffect(() => {
    if (!isCompressibleFileOrUrl && compress) {
      setCompress(false);
    }
  }, [isCompressibleFileOrUrl, compress]);

  // Check if the selected file/URL is a PDF
  const isPDFFileOrUrl = useMemo(() => {
    if (selectedFiles.length > 0) {
      return selectedFiles.every(file => file.name.toLowerCase().endsWith('.pdf'));
    }
    if (selectedFile) {
      return selectedFile.name.toLowerCase().endsWith('.pdf');
    }
    const trimmed = committedUrl.trim();
    if (!trimmed) return false;
    let path = trimmed;
    try {
      path = new URL(trimmed).pathname;
    } catch {
    }
    return path.toLowerCase().endsWith('.pdf');
  }, [selectedFile, selectedFiles, committedUrl]);

  // Show background removal toggle if user has it enabled (multi-user) or config has it enabled (single-user)
  const showBgRemovalToggle = user?.pdf_background_removal || (!multiUserMode && config?.pdf_background_removal);

  useEffect(() => {
    if (!showBgRemovalToggle || !isPDFFileOrUrl) {
      setRemoveBackground(false);
    }
  }, [showBgRemovalToggle, isPDFFileOrUrl]);

  const fetchFoldersWithRefresh = async () => {
    if (isFetchingFolders.current || globalFoldersFetch.isFetching) {
      return;
    }

    const now = Date.now();
    if (now - globalFoldersFetch.lastFetchTime < 1000) {
      return;
    }
    
    isFetchingFolders.current = true;
    globalFoldersFetch.isFetching = true;
    globalFoldersFetch.lastFetchTime = now;
    
    const abortController = new AbortController();
    globalFoldersFetch.abortController = abortController;
    
    try {
      const headers: HeadersInit = {
        "Accept-Language": i18n.language,
      };
      if (uiSecret) {
        headers["X-UI-Token"] = uiSecret;
      }
      
      const res = await fetch("/api/folders?refresh=1", {
        headers,
        credentials: "include",
        signal: abortController.signal,
      }).then((r) => r.json());

      if (Array.isArray(res.folders)) {
        const cleaned = res.folders
          .map((f: string) => f.replace(/^\//, ""))
          .filter((f: string) => f !== "");
        setFolders(cleaned);
      }
    } catch (error: any) {
      if (error.name !== 'AbortError') {
        console.error("Failed to refresh folders:", error);
      }
    } finally {
      isFetchingFolders.current = false;
      globalFoldersFetch.isFetching = false;
    }
  };

  useEffect(() => {
    if (!isAuthenticated || !rmapiPaired || userDataLoading) {
      setFolders([]);
      setFoldersLoading(false);
      return;
    }

    if (hasFetchedInitial.current && refreshTrigger === 0) {
      return;
    }

    if (globalFoldersFetch.currentPromise && globalFoldersFetch.isFetching) {
      globalFoldersFetch.currentPromise.then((foldersData) => {
        if (foldersData) {
          setFolders(foldersData);
          setFoldersLoading(false);
          hasFetchedInitial.current = true;
        }
      }).catch((error) => {
        if (error?.name !== 'AbortError') {
          console.error("Failed to fetch folders:", error);
        }
      });
      return;
    }

    const now = Date.now();
    if (now - globalFoldersFetch.lastFetchTime < 500) {
      return;
    }

    const abortController = new AbortController();
    
    const fetchPromise = (async () => {
      try {
        const headers: HeadersInit = {
          "Accept-Language": i18n.language,
        };
        if (uiSecret) {
          headers["X-UI-Token"] = uiSecret;
        }
        
        const endpoint = refreshTrigger > 0 ? "/api/folders?refresh=1" : "/api/folders";
        
        const res = await fetch(endpoint, {
          headers,
          credentials: "include",
          signal: abortController.signal,
        }).then((r) => r.json());

        if (Array.isArray(res.folders)) {
          const cleaned = res.folders
            .map((f: string) => f.replace(/^\//, ""))
            .filter((f: string) => f !== "");
          
          return cleaned;
        }
        return null;
      } catch (error: any) {
        if (error.name !== 'AbortError') {
          console.error("Failed to fetch folders:", error);
        }
        throw error;
      } finally {
        globalFoldersFetch.currentPromise = null;
        globalFoldersFetch.isFetching = false;
        isFetchingFolders.current = false;
      }
    })();

    globalFoldersFetch.currentPromise = fetchPromise;
    globalFoldersFetch.isFetching = true;
    globalFoldersFetch.lastFetchTime = now;
    globalFoldersFetch.abortController = abortController;
    isFetchingFolders.current = true;

    if (folders.length === 0) {
      setFoldersLoading(true);
    }

    fetchPromise.then((foldersData) => {
      if (foldersData) {
        setFolders(foldersData);
        setFoldersLoading(false);
        hasFetchedInitial.current = true;
      }
    }).catch((error) => {
      if (error?.name !== 'AbortError') {
        setFoldersLoading(false);
      }
    });

    return () => {
      abortController.abort();
      if (globalFoldersFetch.abortController === abortController) {
        globalFoldersFetch.isFetching = false;
        globalFoldersFetch.currentPromise = null;
      }
      isFetchingFolders.current = false;
    };
  }, [isAuthenticated, rmapiPaired, userDataLoading, refreshTrigger]);

  const handlePairingSuccess = () => {
    updatePairingStatus(true);
    setFolders([]);
    setFoldersLoading(true);
    hasFetchedInitial.current = false;
    setTimeout(() => {
      triggerRefresh();
    }, 200);
  };

  // Listen for logout event to clear sensitive state
  useEffect(() => {
    const handleLogout = () => {
      // Clear all sensitive state that could leak between users
      setUrl("");
      setCommittedUrl("");
      setUrlMime(null);
      setSelectedFile(null);
      setSelectedFiles([]);
      setCompress(false);
      setLoading(false);
      setStatus("");
      setMessage("");
      setStatusData(null);
      setProgress(0);
      setUploadProgress(0);
      setUploadPhase('idle');
      setFileError(null);
      setFolders([]);
      setFoldersLoading(false);
      setRmDir(DEFAULT_RM_DIR);
      setPairingDialogOpen(false);
    };

    window.addEventListener('logout', handleLogout);

    return () => {
      window.removeEventListener('logout', handleLogout);
    };
  }, []);

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

  // Helper function to handle WebSocket status updates
  const handleStatusUpdate = (st: JobStatus) => {
    setStatus(st.status.toLowerCase());
    
    // Resolve nested translation keys in the data object
    const resolvedData = st.data ? { ...st.data } : {};
    if (st.data) {
      Object.keys(st.data).forEach(key => {
        const value = st.data[key];
        // If the value looks like a translation key, resolve it
        if (typeof value === 'string' && value.includes('.') && !value.includes(' ')) {
          try {
            const translatedValue = t(value);
            // Only use the translated value if it's different from the key (meaning it was found)
            if (translatedValue !== value) {
              resolvedData[key] = translatedValue;
            }
          } catch (e) {
            // If translation fails, keep the original value
            resolvedData[key] = value;
          }
        }
      });
    }
    
    setMessage(t(st.message, resolvedData));
    setStatusData(st.data || null);
    if (typeof st.progress === "number") {
      setProgress(st.progress);
    }
  };

  const handleSubmit = async () => {
    setLoading(true);
    setMessage("");
    setStatus("");
    // Clear any previous status data when starting new upload
    setStatusData(null);
    setUploadProgress(0);
    setUploadPhase('idle');

    if (selectedFile || selectedFiles.length > 0) {
      try {
        const formData = new FormData();
        
        if (selectedFiles.length > 0) {
          selectedFiles.forEach((file) => {
            formData.append("files", file);
          });
        } else {
          // Single file (legacy)
          formData.append("file", selectedFile!);
        }
        
        formData.append("compress", compress ? "true" : "false");
        if (removeBackground) {
          formData.append("remove_background", "true");
        }
        if (rmDir !== DEFAULT_RM_DIR) {
          formData.append("rm_dir", rmDir);
        }

        setUploadPhase('uploading');
        setMessage(t("home.uploading"));
        const { jobId } = await uploadFileWithProgress(formData);
        
        setUploadPhase('processing');
        setMessage(t("home.job_queued", { id: jobId }));
        setUploadProgress(100);

        setStatus("running");
        setProgress(0);
        await waitForJobWS(jobId, handleStatusUpdate);
      } catch (err: unknown) {
        const msg = getErrorMessage(err);
        setStatus("error");
        setMessage(t(msg));
      } finally {
        setSelectedFile(null);
        setSelectedFiles([]);
        setUrl("");
        setProgress(0);
        setUploadProgress(0);
        setUploadPhase('idle');
        // Don't clear status data immediately - let it persist to show success message properly
        // setStatusData(null);
        setLoading(false);
      }
    } else {
      const form = new URLSearchParams();
      form.append("Body", url);
      form.append("compress", compress ? "true" : "false");
      if (removeBackground) {
        form.append("remove_background", "true");
      }
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

        await waitForJobWS(jobId, handleStatusUpdate);
      } catch (err: unknown) {
        const msg = getErrorMessage(err);
        setStatus("error");
        setMessage(t(msg));
      } finally {
        setUrl("");
        setProgress(0);
        // Don't clear status data immediately - let it persist to show success message properly
        // setStatusData(null);
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
                if (selectedFiles.length > 0) {
                  setSelectedFiles([]);
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
              disabled={!!selectedFile || selectedFiles.length > 0}
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
                if (selectedFiles.length > 0) {
                  setSelectedFiles([]);
                }
                setUrlMime(null);
              }}
              onFilesSelected={(files) => {
                setSelectedFiles(files);
                if (url) {
                  setUrl("");
                }
                if (selectedFile) {
                  setSelectedFile(null);
                }
                setUrlMime(null);
              }}
              onError={(msg) => {
                setFileError(msg);
              }}
              disabled={!!url}
              multiple={true}
              existingFiles={selectedFiles}
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
                  disabled={loading}
                >
                  {t("home.remove")}
                </Button>
              </div>
            )}
            {selectedFiles.length > 0 && (
              <div className="mt-2 space-y-2">
                {selectedFiles.map((file, index) => (
                  <div key={index} className="flex justify-between items-center">
                    <p className="text-sm text-foreground">
                      <span className="font-medium">{file.name}</span>
                    </p>
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => {
                        const newFiles = selectedFiles.filter((_, i) => i !== index);
                        setSelectedFiles(newFiles);
                      }}
                      disabled={loading}
                    >
                      {t("home.remove")}
                    </Button>
                  </div>
                ))}
              </div>
            )}
          </div>

          <div className="space-y-2">
            <Label htmlFor="rmDir">{t("home.destination_folder")}</Label>
            <Select 
              value={rmDir} 
              onValueChange={setRmDir}
              disabled={!rmapiPaired}
              onOpenChange={(open) => {
                if (open && rmapiPaired) {
                  fetchFoldersWithRefresh();
                }
              }}
            >
              <SelectTrigger id="rmDir" className="w-full">
                <SelectValue>
                  {rmDir === DEFAULT_RM_DIR ? t("home.default") : truncateFromStart(rmDir)}
                </SelectValue>
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
                  <SelectItem key={f} value={f} title={f}>
                    {truncateFromStart(f)}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          <div className="grid grid-cols-2 gap-4 mt-4">
            <div className="flex items-center space-x-2">
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
            {showBgRemovalToggle && (
              <div className="flex items-center space-x-2">
                <Label
                  htmlFor="removeBackground"
                  className={!isPDFFileOrUrl ? "opacity-50" : ""}
                >
                  {t("home.remove_background")}
                </Label>
                <Switch
                  id="removeBackground"
                  checked={removeBackground}
                  onCheckedChange={setRemoveBackground}
                  disabled={!isPDFFileOrUrl}
                />
              </div>
            )}
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

          <div className="flex flex-col sm:flex-row sm:justify-end">
            {!userDataLoading && (
              <Button
                onClick={!rmapiPaired ? () => setPairingDialogOpen(true) : handleSubmit}
                disabled={loading || (!url && !selectedFile && selectedFiles.length === 0 && rmapiPaired)}
                className="w-full sm:w-auto"
              >
                {loading ? t("home.sending") : !rmapiPaired ? t("home.pair") : t("home.send")}
              </Button>
            )}
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
{(() => {
                // Check if this is a multiple files success message with structured data
                if (status === "success" && statusData?.paths) {
                  try {
                    const paths = JSON.parse(statusData.paths);
                    if (Array.isArray(paths) && paths.length >= 1) {
                      const messageTemplate = t("backend.status.upload_success_multiple", { paths: "PATHS_PLACEHOLDER" });
                      const parts = messageTemplate.split("PATHS_PLACEHOLDER");
                      const beforePaths = parts[0]?.trim() || "";
                      const afterPaths = parts[1]?.trim() || "";
                      return (
                        <div className="break-words">
                          {beforePaths && <div className="mb-1">{beforePaths}</div>}
                          <ul className="list-disc list-outside ml-6 sm:ml-5 mt-1 space-y-1">
                            {paths.map((path, index) => (
                              <li key={index} className="text-sm leading-relaxed">{path}</li>
                            ))}
                          </ul>
                          {afterPaths && <div className="mt-1">{afterPaths}</div>}
                        </div>
                      );
                    }
                  } catch (e) {
                    console.error('Failed to parse paths JSON:', e, statusData?.paths);
                  }
                }
                
                // Default text rendering
                return <span className="break-words whitespace-pre-line">{message}</span>;
              })()}
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

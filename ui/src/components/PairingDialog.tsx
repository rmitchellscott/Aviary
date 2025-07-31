import React, { useState } from "react";
import { useTranslation } from "react-i18next";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { useAuth } from "@/components/AuthProvider";

interface PairingDialogProps {
  isOpen: boolean;
  onClose: () => void;
  onPairingSuccess: () => void;
  rmapiHost?: string;
}

export function PairingDialog({ 
  isOpen, 
  onClose, 
  onPairingSuccess,
  rmapiHost = ""
}: PairingDialogProps) {
  const { t } = useTranslation();
  const [code, setCode] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleClose = () => {
    setCode("");
    setError(null);
    setLoading(false);
    onClose();
  };

  const { multiUserMode } = useAuth();

  const handleSubmit = async () => {
    const trimmedCode = code.trim();
    
    if (trimmedCode.length !== 8) {
      setError(t("pairing.code_error"));
      return;
    }

    setLoading(true);
    setError(null);
    
    try {
      // Use different endpoint based on mode
      const endpoint = multiUserMode ? "/api/profile/pair" : "/api/pair";
      
      const resp = await fetch(endpoint, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        credentials: "include",
        body: JSON.stringify({ code: trimmedCode }),
      });
      
      if (resp.ok) {
        onPairingSuccess();
        handleClose();
      } else {
        const data = await resp.json();
        setError(data.error || t("pairing.pair_error"));
      }
    } catch {
      setError(t("pairing.pair_error"));
    } finally {
      setLoading(false);
    }
  };

  const handleCodeChange = (value: string) => {
    // Only allow alphanumeric characters and limit to 8
    const cleanValue = value.replace(/[^a-zA-Z0-9]/g, '').slice(0, 8);
    setCode(cleanValue);
    setError(null);
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && code.length === 8 && !loading) {
      handleSubmit();
    }
  };

  const displayHost = rmapiHost && rmapiHost.trim() !== "" ? rmapiHost : "my.remarkable.com";

  return (
    <Dialog open={isOpen} onOpenChange={handleClose}>
      <DialogContent className="sm:max-w-md max-h-[60vh] overflow-y-auto max-sm:top-[20vh]">
        <DialogHeader>
          <DialogTitle>{t("pairing.title")}</DialogTitle>
          <DialogDescription>
            {t("pairing.description", { displayHost })}
          </DialogDescription>
        </DialogHeader>
        
        <div className="space-y-4">
          <div>
            <Label htmlFor="pairing-code">{t("pairing.code_label")}</Label>
            <Input
              id="pairing-code"
              value={code}
              onChange={(e) => handleCodeChange(e.target.value)}
              onKeyDown={handleKeyDown}
              placeholder={t("pairing.code_placeholder")}
              className="font-mono text-center tracking-widest mt-2"
              maxLength={8}
              disabled={loading}
            />
          </div>
          
          {error && (
            <div className="text-sm text-destructive bg-destructive/10 border border-destructive/20 p-2 rounded">
              {error}
            </div>
          )}
          
          <div className="flex justify-end space-x-2">
            <Button
              variant="outline"
              onClick={handleClose}
              disabled={loading}
            >
              {t("pairing.cancel")}
            </Button>
            <Button
              onClick={handleSubmit}
              disabled={loading || code.length !== 8}
            >
              {loading ? (
                <>
                  <div className="animate-spin rounded-full h-4 w-4 border-b-2 border-primary-foreground mr-2"></div>
                  {t("pairing.pairing")}
                </>
              ) : (
                t("pairing.pair")
              )}
            </Button>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  );
}

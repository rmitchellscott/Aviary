'use client'

import { useState, useEffect } from 'react'
import {useMemo} from 'react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { FileDropzone } from '@/components/FileDropzone'

/**
 * Helper to turn any thrown value into a string.
 */
function getErrorMessage(err: unknown): string {
  if (err instanceof Error) {
    return err.message
  }
  return String(err)
}

export default function HomePage() {
  const [url, setUrl] = useState<string>('')
  const [committedUrl, setCommittedUrl] = useState<string>('')
  const [selectedFile, setSelectedFile] = useState<File | null>(null)
  const [compress, setCompress] = useState<boolean>(false)
  const [loading, setLoading] = useState<boolean>(false)
  const [message, setMessage] = useState<string>('')

  /**
   * Determine if “Compress PDF” should be enabled:
   * - File mode: only if selected file name ends with “.pdf”
   * - URL mode: if URL ends with “.pdf”; if URL has other extension, disable; if no extension, keep enabled.
   */
  const isPdfFileOrUrl = useMemo(() => {
    if (selectedFile) {
      return selectedFile.name.toLowerCase().endsWith('.pdf')
    }
    const trimmed = url.trim().toLowerCase()
    if (!trimmed) {
      // No URL entered → allow compress switch (it’s harmless if clicked before submit)
      return true
    }
    // If URL ends with .pdf, enable
    if (trimmed.endsWith('.pdf')) {
      return true
    }
    // Check for any other extension in the last path segment
    // e.g. if URL contains “/file.txt” or “/file.docx”, disable.
    const lastSegment = trimmed.split('/').pop() || ''
    if (lastSegment.includes('.') && !lastSegment.endsWith('.pdf')) {
      return false
    }
    // No extension in URL (e.g. “https://example.com/download”), allow compress
    return true
  }, [selectedFile, committedUrl])

  useEffect(() => {
    if (!isPdfFileOrUrl && compress) {
      setCompress(false)
    }
  }, [isPdfFileOrUrl, compress])

  /**
   * If a local file is selected, POST it to /api/upload as multipart/form-data.
   * Otherwise, enqueue by sending a URL to /api/webhook (old behavior).
   */
  const handleSubmit = async () => {
    setLoading(true)
    setMessage('')

  if (selectedFile) {
    // === FILE UPLOAD FLOW (enqueue + poll) ===
    try {
      const formData = new FormData()
      formData.append('file', selectedFile)
      formData.append('compress', compress ? 'true' : 'false')

      // 1) send to /api/upload and get back { jobId }
      const res = await fetch('/api/upload', {
        method: 'POST',
        body: formData,
      })
      if (!res.ok) {
        const errText = await res.text()
        throw new Error(`Upload failed: ${errText}`)
      }
      const { jobId } = await res.json()
      setMessage(`Job queued: ${jobId}`)

      // 2) poll exactly as in the URL flow
      let done = false
      while (!done) {
        await new Promise((r) => setTimeout(r, 1500))
        const st = await fetch(`/api/status/${jobId}`).then((r) => r.json())
        setMessage(`${st.status}: ${st.message}`)
        if (st.status === 'success') {
          setMessage(st.message)
          done = true
        } else if (st.status === 'error') {
          setMessage(`❌ ${st.message}`)
          done = true
        }
      }
    } catch (err: unknown) {
      const msg = getErrorMessage(err)
      setMessage(`❌ ${msg}`)
    } finally {
      // clear the selected file so <Input> becomes enabled again
      setSelectedFile(null)
      setUrl('')
      setLoading(false)
    }
  } else {
      // === URL SUBMIT FLOW (EXISTING) ===
      const form = new URLSearchParams()
      form.append('Body', url)
      form.append('compress', compress ? 'true' : 'false')

      try {
        const res = await fetch('/api/webhook', {
          method: 'POST',
          headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
          body: form.toString(),
        })
        if (!res.ok) {
          const errText = await res.text()
          throw new Error(`Enqueue failed: ${errText}`)
        }
        const { jobId } = await res.json()
        setMessage(`Job queued: ${jobId}`)

        // poll loop
        let done = false
        while (!done) {
          await new Promise((r) => setTimeout(r, 1500))
          const st = await fetch(`/api/status/${jobId}`).then((r) => r.json())
          setMessage(`${st.status}: ${st.message}`)
          if (st.status === 'success') {
            setMessage(st.message)
            done = true
          } else if (st.status === 'error') {
            setMessage(`❌ ${st.message}`)
            done = true
          }
        }
      } catch (err: unknown) {
        const msg = getErrorMessage(err)
        setMessage(`❌ ${msg}`)
      } finally {
        setUrl('')
        setLoading(false)
      }
    }
  }

  return (
    <div className="bg-background pt-0 pb-8 px-8">

      <Card className="max-w-md mx-auto bg-card">
        <CardHeader>
          <CardTitle className="text-xl">Send Document</CardTitle>
        </CardHeader>

        <CardContent className="space-y-6">
          {/* === URL INPUT === */}
          <div>
            <Input
              id="url"
              type="text"
              value={url}
              onChange={(e) => {
                setUrl(e.target.value)
                // Clear any selected file if the user starts typing a URL
                if (selectedFile) {
                  setSelectedFile(null)
                }
              }}
            onBlur={() => {
              // commit the URL once the user leaves the field
              setCommittedUrl(url)
            }}
              placeholder="https://example.com/file.pdf"
              disabled={!!selectedFile}
            />
          </div>

          <div className="text-center text-sm text-muted-foreground">— OR —</div>

          {/* === DRAG & DROP FILE === */}
          <div>
            <FileDropzone
              onFileSelected={(file) => {
                setSelectedFile(file)
                // Clear any URL if the user picks a file
                if (url) {
                  setUrl('')
                }
              }}
              disabled={!!url}
            />
          {selectedFile && (
            <div className="mt-2 flex justify-between items-center">
              <p className="text-sm text-foreground">
                Selected file: <span className="font-medium">{selectedFile.name}</span>
              </p>
              <button
                onClick={() => setSelectedFile(null)}
                className="text-sm text-muted-foreground hover:text-foreground"
              >
                <b>Remove</b>
              </button>
            </div>
          )}
          </div>

          {/* === COMPRESS SWITCH === */}
        <div className="flex items-center space-x-2">
          <Switch
            id="compress"
            checked={compress}
            onCheckedChange={setCompress}
            disabled={!isPdfFileOrUrl}
          />
          <Label htmlFor="compress" className={!isPdfFileOrUrl ? 'opacity-50' : ''}>
            Compress PDF
          </Label>
        </div>

          {/* === SUBMIT BUTTON === */}
          <div className="flex justify-end">
            <Button onClick={handleSubmit} disabled={loading || (!url && !selectedFile)}>
              {loading ? 'Sending…' : 'Send'}
            </Button>
          </div>

          {message && (
            <p className="mt-2 text-sm text-muted-foreground break-words">{message}</p>
          )}
        </CardContent>
      </Card>
    </div>
  )
}

'use client'

import { useState } from 'react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
// import { Separator } from '@/components/ui/separator'

export default function HomePage() {
  const [url, setUrl] = useState('')
  const [compress, setCompress] = useState(false)
  const [loading, setLoading] = useState(false)
  const [message, setMessage] = useState('')

  const handleSubmit = async () => {
    setLoading(true)
    setMessage('')

    // enqueue
    const form = new URLSearchParams()
    form.append('Body', url)
    form.append('compress', compress ? 'true' : 'false')
    // …other fields…

    const res = await fetch('/api/webhook', {
      method: 'POST',
      headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
      body: form.toString(),
    })
    const { jobId } = await res.json()
    setMessage(`Job queued: ${jobId}`)

    // poll loop
    let done = false
    while (!done) {
      await new Promise(r => setTimeout(r, 1500))
      const st = await fetch(`/api/status/${jobId}`).then(r => r.json())
      setMessage(`${st.status}: ${st.message}`)
      if (st.status === 'success') {
        setMessage(st.message)         // no “success:” prefix
        done = true
      } else if (st.status === 'error') {
        setMessage(`❌ ${st.message}`)  // keep an error indicator
        done = true
      }
    }

    setLoading(false)
  }
      // if (st.status === 'success' || st.status === 'error') {
      //   done = true
      // }
  //   }

  //   setLoading(false)
  // }

  // const handleSubmit = async () => {
  //   setLoading(true)
  //   setMessage('')
  //   try {
  //     const form = new URLSearchParams()
  //     form.append('Body', url)
  //     form.append('compress', compress ? 'true' : 'false')

  //     const res = await fetch('/api/webhook', {
  //       method: 'POST',
  //       headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
  //       body: form.toString(),
  //     })

  //     if (res.ok) {
  //       setMessage('✅ Request accepted')
  //       setUrl('')
  //       setCompress(false)
  //     } else {
  //       const err = await res.json()
  //       setMessage(`❌ Error: ${err.message}`)
  //     }
  //   } catch {
  //     setMessage('🚫 Network error')
  //   } finally {
  //     setLoading(false)
  //   }
  // }

  return (
    <div className="min-h-screen bg-background p-8">
      <header className="mb-8">
        <h1 className="text-3xl font-bold">Aviary</h1>
      </header>

      <Card className="max-w-md mx-auto bg-card">
        <CardHeader>
          <CardTitle className="text-xl">Upload Document</CardTitle>
        </CardHeader>
        {/* <Separator /> */}
        <CardContent className="space-y-6">
          <div>
            <Input
              id="url"
              type="text"
              value={url}
              onChange={e => setUrl(e.target.value)}
              placeholder="https://example.com/file.pdf"
            />
          </div>

          <div className="flex items-center space-x-2">
            <Switch
              id="compress"
              checked={compress}
              onCheckedChange={setCompress}
            />
            <Label htmlFor="compress">Compress PDF</Label>
          </div>

          <Button
            onClick={handleSubmit}
            disabled={loading || !url}
          >
            {loading ? 'Sending…' : 'Submit'}
          </Button>

          {message && (
            <p className="text-sm text-muted-foreground">{message}</p>
          )}
        </CardContent>
      </Card>
    </div>
  )
}

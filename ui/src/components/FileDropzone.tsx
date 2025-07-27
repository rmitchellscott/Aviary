'use client'

import { useCallback, useEffect, useState } from 'react'
import { useDropzone, FileRejection } from 'react-dropzone'
import { useTranslation } from 'react-i18next'

interface FileDropzoneProps {
  onFileSelected: (file: File) => void
  onFilesSelected: (files: File[]) => void
  disabled?: boolean
  onError?: (message: string) => void
  multiple?: boolean
}

/**
 * A ShadCN/Tailwind-themed drag-and-drop box for picking exactly one file.
 * When the user drops or selects a file, it calls onFileSelected(file).
 * If the file is rejected (wrong extension/MIME), it calls onError(msg).
 *
 * - Default: dashed border in `border-input`, transparent background (so it “sits” on bg-card).
 * - Hover: borders change to `border-primary`.
 * - Drag active: `border-primary` + `bg-muted`, text in `text-foreground`.
 * - Disabled: `opacity-50`, `cursor-not-allowed`, still uses `border-input`.
 */
export function FileDropzone({
  onFileSelected,
  onFilesSelected,
  disabled = false,
  onError,
  multiple = false,
}: FileDropzoneProps) {
  const { t } = useTranslation()
  const onDrop = useCallback(
    (acceptedFiles: File[], fileRejections: FileRejection[]) => {
      // If any files were rejected, show the first rejection message
      if (fileRejections.length > 0) {
        // Show a user-friendly list of allowed types:
        if (onError) {
          onError(t('filedrop.invalid_type'))
        }
        return
      }

      // Otherwise, accept files based on multiple mode
      if (acceptedFiles.length > 0) {
        if (multiple) {
          onFilesSelected(acceptedFiles)
        } else {
          onFileSelected(acceptedFiles[0])
        }
      }
    },
    [onFileSelected, onFilesSelected, onError, multiple]
  )

  const { getRootProps, getInputProps, isDragActive } = useDropzone({
    onDrop,
    multiple,
    disabled,
    accept: {
      'application/pdf': ['.pdf'],
      'application/epub+zip': ['.epub'],
      'image/jpeg': ['.jpg', '.jpeg'],
      'image/png': ['.png'],
    },
  })

  // Track drag events occurring anywhere on the window so the dropzone becomes
  // active even when a file is dragged outside the element itself.
  const [windowDragActive, setWindowDragActive] = useState(false)
  useEffect(() => {
    if (disabled) return

    let counter = 0
    function handleDragEnter(e: DragEvent) {
      if (Array.from(e.dataTransfer?.types || []).includes('Files')) {
        counter++
        setWindowDragActive(true)
      }
    }
    function handleDragLeave() {
      counter = Math.max(counter - 1, 0)
      if (counter === 0) setWindowDragActive(false)
    }
    function handleDragOver(e: DragEvent) {
      // Prevent default to allow drop
      e.preventDefault()
    }
    function handleDrop(e: DragEvent) {
      e.preventDefault()
      counter = 0
      setWindowDragActive(false)
      
      // Handle the drop anywhere on the page
      const files = Array.from(e.dataTransfer?.files || [])
      if (files.length > 0) {
        // Use the same validation logic as react-dropzone
        const file = files[0]
        const acceptedTypes = [
          'application/pdf',
          'application/epub+zip', 
          'image/jpeg',
          'image/png'
        ]
        const acceptedExtensions = ['.pdf', '.epub', '.jpg', '.jpeg', '.png']
        
        const isValidType = acceptedTypes.includes(file.type)
        const isValidExtension = acceptedExtensions.some(ext => 
          file.name.toLowerCase().endsWith(ext)
        )
        
        if (isValidType || isValidExtension) {
          if (multiple) {
            onFilesSelected([file])
          } else {
            onFileSelected(file)
          }
        } else if (onError) {
          onError(t('filedrop.invalid_type'))
        }
      }
    }

    window.addEventListener('dragenter', handleDragEnter)
    window.addEventListener('dragleave', handleDragLeave)
    window.addEventListener('dragover', handleDragOver)
    window.addEventListener('drop', handleDrop)
    return () => {
      window.removeEventListener('dragenter', handleDragEnter)
      window.removeEventListener('dragleave', handleDragLeave)
      window.removeEventListener('dragover', handleDragOver)
      window.removeEventListener('drop', handleDrop)
    }
  }, [disabled, onFileSelected, onFilesSelected, onError, t, multiple])

  const active = isDragActive || windowDragActive

  return (
    <div
      {...getRootProps()}
      className={
        'border-2 border-dashed rounded-lg p-6 text-center cursor-pointer transition-colors ' +
        (disabled
          ? 'opacity-50 cursor-not-allowed border-input'
          : active
          ? 'border-primary bg-muted text-foreground'
          : 'border-input hover:border-primary text-muted-foreground')
      }
    >
      <input {...getInputProps()} />
      {active ? (
        <p className="text-sm">{t('filedrop.instruction')}</p>
      ) : (
        <p className="text-sm">{t('filedrop.instruction')}</p>
      )}
    </div>
  )
}

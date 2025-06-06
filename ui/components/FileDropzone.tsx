'use client'

import { useCallback } from 'react'
import { useDropzone, FileRejection } from 'react-dropzone'

interface FileDropzoneProps {
  onFileSelected: (file: File) => void
  disabled?: boolean
  onError?: (message: string) => void
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
  disabled = false,
  onError,
}: FileDropzoneProps) {
  const onDrop = useCallback(
    (acceptedFiles: File[], fileRejections: FileRejection[]) => {
      // If any files were rejected, show the first rejection message
      if (fileRejections.length > 0) {
        // Show a user-friendly list of allowed types:
        if (onError) {
          onError('Please select a PDF, EPUB, JPEG, or a PNG file.')
        }
        return
      }
    //   if (fileRejections.length > 0) {
    //     const firstRej = fileRejections[0]
    //     const msg = firstRej.errors[0]?.message || 'Invalid file type'
    //     if (onError) {
    //       onError(msg)
    //     }
    //     return
    //   }

      // Otherwise, accept exactly the first file
      if (acceptedFiles.length > 0) {
        onFileSelected(acceptedFiles[0])
      }
    },
    [onFileSelected, onError]
  )

  const { getRootProps, getInputProps, isDragActive } = useDropzone({
    onDrop,
    multiple: false,
    disabled,
    accept: {
      'application/pdf': ['.pdf'],
      'application/epub+zip': ['.epub'],
      'image/jpeg': ['.jpg', '.jpeg'],
      'image/png': ['.png'],
    },
  })

  return (
    <div
      {...getRootProps()}
      className={
        'border-2 border-dashed rounded-lg p-6 text-center cursor-pointer transition-colors ' +
        (disabled
          ? 'opacity-50 cursor-not-allowed border-input'
          : isDragActive
          ? 'border-primary bg-muted text-foreground'
          : 'border-input hover:border-primary text-muted-foreground')
      }
    >
      <input {...getInputProps()} />
      {isDragActive ? (
        <p className="text-sm">
          <b>Click to upload</b> or drag and drop
        </p>
      ) : (
        <p className="text-sm">
          <b>Click to upload</b> or drag and drop
        </p>
      )}
    </div>
  )
}

// ui/components/FileDropzone.tsx
'use client'

import { useCallback } from 'react'
import { useDropzone } from 'react-dropzone'

interface FileDropzoneProps {
  onFileSelected: (file: File) => void
  disabled?: boolean
}

/**
 * A ShadCN/Tailwind‐themed drag‐and‐drop box for picking exactly one file.
 * When the user drops or selects a file, it calls onFileSelected(file).
 * 
 * - Default: dashed border in `border-input`, transparent background (so it “sits” on bg-card).
 * - Hover: blooders change to `border-primary`.
 * - Drag active: `border-primary` + `bg-muted`, text in `text-foreground`.
 * - Disabled: `opacity-50`, `cursor-not-allowed`, still uses `border-input`.
 */
export function FileDropzone({ onFileSelected, disabled = false }: FileDropzoneProps) {
  const onDrop = useCallback(
    (acceptedFiles: File[]) => {
      if (acceptedFiles && acceptedFiles.length > 0) {
        onFileSelected(acceptedFiles[0])
      }
    },
    [onFileSelected]
  )

  const { getRootProps, getInputProps, isDragActive } = useDropzone({
    onDrop,
    multiple: false,
    disabled,
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
        <p className="text-sm"><b>Click to upload</b> or drag and drop</p>
      ) : (
        <p className="text-sm"><b>Click to upload</b> or drag and drop</p>
      )}
    </div>
  )
}

'use client'

import { useCallback, useEffect, useState, useRef } from 'react'
import { useTranslation } from 'react-i18next'

interface FileDropzoneProps {
  onFileSelected: (file: File) => void
  onFilesSelected: (files: File[]) => void
  disabled?: boolean
  onError?: (message: string) => void
  multiple?: boolean
  existingFiles?: File[]
}

const ACCEPT_CONFIG = {
  'application/pdf': ['.pdf'],
  'application/epub+zip': ['.epub'],
  'image/jpeg': ['.jpg', '.jpeg'],
  'image/png': ['.png'],
}

/**
 * A unified drag-and-drop component with single handler for both visible zone and whole-page drops.
 */
export function FileDropzone({
  onFileSelected,
  onFilesSelected,
  disabled = false,
  onError,
  multiple = false,
  existingFiles = [],
}: FileDropzoneProps) {
  const { t } = useTranslation()
  const fileInputRef = useRef<HTMLInputElement>(null)
  const [dragActive, setDragActive] = useState(false)
  
  // Unified file processing function
  const processFiles = useCallback(
    (files: File[]) => {
      const acceptedFiles: File[] = []
      const rejectedFiles: File[] = []
      
      files.forEach(file => {
        const acceptedTypes = Object.keys(ACCEPT_CONFIG)
        const acceptedExtensions = Object.values(ACCEPT_CONFIG).flat()
        
        const isValidType = acceptedTypes.includes(file.type)
        const isValidExtension = acceptedExtensions.some(ext => 
          file.name.toLowerCase().endsWith(ext)
        )
        
        if (isValidType || isValidExtension) {
          acceptedFiles.push(file)
        } else {
          rejectedFiles.push(file)
        }
      })
      
      if (rejectedFiles.length > 0 && onError) {
        onError(t('filedrop.invalid_type'))
        return
      }

      if (acceptedFiles.length > 0) {
        if (multiple) {
          // Combine existing files with new accepted files
          const combinedFiles = [...existingFiles, ...acceptedFiles]
          onFilesSelected(combinedFiles)
        } else {
          onFileSelected(acceptedFiles[0])
        }
      }
    },
    [onFileSelected, onFilesSelected, onError, multiple, t, existingFiles]
  )

  // Single unified handler for all drag/drop events
  useEffect(() => {
    if (disabled) return

    let counter = 0
    
    function handleDragEnter(e: DragEvent) {
      if (Array.from(e.dataTransfer?.types || []).includes('Files')) {
        counter++
        setDragActive(true)
      }
    }
    
    function handleDragLeave() {
      counter = Math.max(counter - 1, 0)
      if (counter === 0) setDragActive(false)
    }
    
    function handleDragOver(e: DragEvent) {
      e.preventDefault()
    }
    
    function handleDrop(e: DragEvent) {
      e.preventDefault()
      counter = 0
      setDragActive(false)
      
      const files = Array.from(e.dataTransfer?.files || [])
      if (files.length > 0) {
        const filesToProcess = multiple ? files : [files[0]]
        processFiles(filesToProcess)
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
  }, [disabled, processFiles, multiple])

  const handleClick = () => {
    if (!disabled && fileInputRef.current) {
      fileInputRef.current.click()
    }
  }

  const handleFileInputChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const files = Array.from(e.target.files || [])
    if (files.length > 0) {
      processFiles(files)
    }
    // Reset input value to allow selecting the same file again
    e.target.value = ''
  }

  return (
    <div
      onClick={handleClick}
      className={
        'border-2 border-dashed rounded-lg p-6 text-center cursor-pointer transition-colors ' +
        (disabled
          ? 'opacity-50 cursor-not-allowed border-input'
          : dragActive
          ? 'border-primary bg-muted text-foreground'
          : 'border-input hover:border-primary text-muted-foreground')
      }
    >
      <input
        ref={fileInputRef}
        type="file"
        multiple={multiple}
        accept={Object.values(ACCEPT_CONFIG).flat().join(',')}
        onChange={handleFileInputChange}
        className="hidden"
        disabled={disabled}
      />
      <p className="text-sm">{t('filedrop.instruction')}</p>
    </div>
  )
}

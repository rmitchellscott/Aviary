"use client"

import { useState, useRef, useEffect } from "react"
import { useTranslation } from "react-i18next"
import { Globe, Check } from "lucide-react"
import i18n from "@/lib/i18n"

import { Button } from "@/components/ui/button"
import { Popover, PopoverTrigger, PopoverContent } from "@/components/ui/popover"
import {
  Command,
  CommandInput,
  CommandList,
  CommandEmpty,
  CommandItem,
} from "@/components/ui/command"
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip"

// Dynamically generate language list from available i18n resources
const getAvailableLanguages = () => {
  const availableLanguages = Object.keys(i18n.options.resources || {})
  return availableLanguages.map(lang => {
    const resource = i18n.options.resources?.[lang]?.translation as any
    const displayName = resource?._meta?.displayName || lang.toUpperCase()
    return {
      value: lang,
      label: displayName
    }
  }).sort((a, b) => a.label.localeCompare(b.label))
}

export default function LanguageSwitcher() {
  const { i18n, t } = useTranslation()
  const current = i18n.resolvedLanguage || i18n.language.split("-")[0]
  const [open, setOpen] = useState(false)
  const availableLanguages = getAvailableLanguages()
  const searchRef = useRef<HTMLInputElement>(null)
  const isIOS = typeof window !== 'undefined' && /iPad|iPhone|iPod/.test(navigator.userAgent)

  useEffect(() => {
    if (open && !isIOS) {
      searchRef.current?.focus()
    }
  }, [open, isIOS])

  const handleLanguageChange = (langValue: string) => {
    i18n.changeLanguage(langValue)
    setOpen(false)
  }

  return (
    <TooltipProvider>
      <Tooltip>
        <Popover open={open} onOpenChange={setOpen}>
          <TooltipTrigger asChild>
            <PopoverTrigger asChild>
              <Button 
                variant="ghost" 
                size="icon" 
                aria-label="Select language"
                style={{
                  fontSize: 21,
                  lineHeight: 0,
                }}
              >
                <Globe className="text-muted-foreground" style={{ width: '1em', height: '1em' }} />
              </Button>
            </PopoverTrigger>
          </TooltipTrigger>
      <PopoverContent
        className="w-40 p-0"
        onOpenAutoFocus={(e) => {
          if (isIOS) e.preventDefault()
        }}
      >
        <Command>
          <CommandInput
            ref={searchRef}
            placeholder={t("language.search") || "Search languages..."}
            className="h-8"
          />
          <CommandList>
            <CommandEmpty>{t("language.no_results") || "No languages found."}</CommandEmpty>
            {availableLanguages.map((lang) => (
              <CommandItem
                key={lang.value}
                value={lang.label}
                onSelect={() => handleLanguageChange(lang.value)}
                className="cursor-pointer"
                style={{ pointerEvents: 'auto' }}
              >
                <span className="flex items-center w-full">
                  {lang.label}
                  {current === lang.value && <Check className="ml-auto size-4" />}
                </span>
              </CommandItem>
            ))}
          </CommandList>
        </Command>
      </PopoverContent>
        </Popover>
        <TooltipContent>
          <p>{t("language.tooltip") || "Language"}</p>
        </TooltipContent>
      </Tooltip>
    </TooltipProvider>
  )
}

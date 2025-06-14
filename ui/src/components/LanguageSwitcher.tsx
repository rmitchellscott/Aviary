"use client"

import { useState } from "react"
import { useTranslation } from "react-i18next"
import { Globe, Check } from "lucide-react"

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

const LANGS = [
  { value: "en", label: "English" },
  { value: "es", label: "Español" },
]

export default function LanguageSwitcher() {
  const { i18n, t } = useTranslation()
  const current = i18n.resolvedLanguage || i18n.language.split("-")[0]
  const [open, setOpen] = useState(false)

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
              <Button variant="ghost" size="icon" aria-label="Select language">
                <Globe className="size-4" />
              </Button>
            </PopoverTrigger>
          </TooltipTrigger>
      <PopoverContent className="w-40 p-0">
        <Command>
          <CommandInput placeholder={t("language.search") || "Search languages..."} className="h-8" />
          <CommandList>
            <CommandEmpty>{t("language.no_results") || "No languages found."}</CommandEmpty>
            {LANGS.map((lang) => (
              <CommandItem
                key={lang.value}
                value={lang.label}
                onSelect={() => handleLanguageChange(lang.value)}
                className="cursor-pointer"
                onClick={(e) => {
                  e.stopPropagation()
                  e.preventDefault()
                  handleLanguageChange(lang.value)
                }}
                onPointerDown={(e) => {
                  e.stopPropagation()
                  e.preventDefault()
                  handleLanguageChange(lang.value)
                }}
                style={{ pointerEvents: 'auto' }}
              >
                <span 
                  onClick={(e) => {
                    e.stopPropagation()
                    e.preventDefault()
                    handleLanguageChange(lang.value)
                  }}
                  className="flex items-center w-full"
                >
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

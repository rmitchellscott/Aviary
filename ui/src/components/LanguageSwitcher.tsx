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

const LANGS = [
  { value: "en", label: "English" },
  { value: "es", label: "Español" },
]

export default function LanguageSwitcher() {
  const { i18n, t } = useTranslation()
  const current = i18n.resolvedLanguage || i18n.language.split("-")[0]
  const [open, setOpen] = useState(false)

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <Button variant="ghost" size="icon" aria-label="Select language">
          <Globe className="size-4" />
        </Button>
      </PopoverTrigger>
      <PopoverContent className="w-40 p-0">
        <Command>
          <CommandInput placeholder={t("language.search") || "Search..."} className="h-8" />
          <CommandList>
            <CommandEmpty>{t("language.no_results") || "No results."}</CommandEmpty>
            {LANGS.map((lang) => (
              <CommandItem
                key={lang.value}
                onSelect={() => {
                  i18n.changeLanguage(lang.value)
                  setOpen(false)
                }}
              >
                {lang.label}
                {current === lang.value && <Check className="ml-auto size-4" />}
              </CommandItem>
            ))}
          </CommandList>
        </Command>
      </PopoverContent>
    </Popover>
  )
}

import i18n from 'i18next';
import { initReactI18next } from 'react-i18next';
import LanguageDetector from 'i18next-browser-languagedetector';

// Import all translation files
// NOTE: When adding a new language, add two lines:
// 1. import statement here
// 2. entry in the resources object below
import en from '@locales/en.json';
import es from '@locales/es.json';
import de from '@locales/de.json';
import fr from '@locales/fr.json';
import nl from '@locales/nl.json';
import it from '@locales/it.json';
import pt from '@locales/pt.json';
import no from '@locales/no.json';
import sv from '@locales/sv.json';
import da from '@locales/da.json';
import fi from '@locales/fi.json';
import pl from '@locales/pl.json';
import ja from '@locales/ja.json';
import ko from '@locales/ko.json';
import zhCN from '@locales/zh-CN.json';

i18n
  .use(LanguageDetector)
  .use(initReactI18next)
  .init({
    resources: {
      // NOTE: When adding a new language, add an entry here:
      // xx: { translation: xx },
      en: { translation: en },
      es: { translation: es },
      de: { translation: de },
      fr: { translation: fr },
      nl: { translation: nl },
      it: { translation: it },
      pt: { translation: pt },
      no: { translation: no },
      sv: { translation: sv },
      da: { translation: da },
      fi: { translation: fi },
      pl: { translation: pl },
      ja: { translation: ja },
      ko: { translation: ko },
      'zh-CN': { translation: zhCN },
    },
    fallbackLng: 'en',
    interpolation: {
      escapeValue: false,
    },
    detection: {
      order: ['localStorage', 'navigator'],
      caches: ['localStorage'],
      lookupLocalStorage: 'i18nextLng',
    },
  });

export default i18n;

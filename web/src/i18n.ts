import i18n, { BackendModule, FallbackLng, FallbackLngObjList } from "i18next";
import { orderBy } from "lodash-es";
import { initReactI18next } from "react-i18next";
import { findNearestMatchedLanguage } from "./utils/i18n";

export const locales = orderBy([
  "ar",
  "az",
  "bg",
  "ca",
  "cs",
  "da",
  "de",
  "el",
  "en",
  "en-GB",
  "es",
  "et",
  "fa",
  "fi",
  "fr",
  "gl",
  "he",
  "hi",
  "hr",
  "hu",
  "id",
  "it",
  "ja",
  "ka-GE",
  "ko",
  "lt",
  "lv",
  "mr",
  "nb",
  "nl",
  "pl",
  "pt-PT",
  "pt-BR",
  "ro",
  "ru",
  "sk",
  "sl",
  "sr",
  "sv",
  "th",
  "tr",
  "uk",
  "vi",
  "zh-Hans",
  "zh-Hant",
]);

const fallbacks = {
  "zh-HK": ["zh-Hant", "en"],
  "zh-TW": ["zh-Hant", "en"],
  zh: ["zh-Hans", "en"],
} as FallbackLngObjList;

export const getInitialLocale = (): string => {
  try {
    // Existing installations may have an English preference from before Connla became Chinese-first.
    // Apply the new default once; later explicit language changes remain persistent.
    if (!localStorage.getItem("connla-chinese-default-v1")) {
      localStorage.setItem("connla-locale", "zh-Hans");
      localStorage.setItem("connla-chinese-default-v1", "done");
      return "zh-Hans";
    }
    const stored = localStorage.getItem("connla-locale");
    return stored && locales.includes(stored) ? stored : "zh-Hans";
  } catch {
    return "zh-Hans";
  }
};

const LazyImportPlugin: BackendModule = {
  type: "backend",
  init: function () {},
  read: function (language, _, callback) {
    const matchedLanguage = findNearestMatchedLanguage(language);
    import(`./locales/${matchedLanguage}.json`)
      .then((translationModule: Record<string, unknown>) => {
        callback(null, (translationModule.default as Record<string, unknown>) ?? translationModule);
      })
      .catch(() => {
        import("./locales/zh-Hans.json")
          .then((translationModule: Record<string, unknown>) => {
            callback(null, (translationModule.default as Record<string, unknown>) ?? translationModule);
          })
          .catch((error: unknown) => {
            callback(error as Error, false);
          });
      });
  },
};

i18n
  .use(LazyImportPlugin)
  .use(initReactI18next)
  .init({
    lng: getInitialLocale(),
    detection: {
      order: ["navigator"],
    },
    interpolation: {
      escapeValue: false,
    },
    fallbackLng: {
      ...fallbacks,
      ...{ default: ["zh-Hans"] },
    } as FallbackLng,
  });

export default i18n;
export type TLocale = (typeof locales)[number];

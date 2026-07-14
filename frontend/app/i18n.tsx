"use client";

import { createContext, ReactNode, useContext, useEffect, useState } from "react";

export type Locale = "en" | "pt-BR";

const ptBR: Record<string, string> = {
  "language.english": "English",
  "language.portuguese": "Português (Brasil)",
  "language.label": "Idioma",
  "common.loading": "Carregando...",
  "common.backDashboard": "Voltar ao painel",
  "common.notAvailable": "Não disponível",
  "auth.username": "Usuário",
  "auth.password": "Senha",
  "auth.signIn": "Entrar",
  "auth.signingIn": "Entrando...",
  "auth.signOut": "Sair",
  "auth.createAccount": "Criar conta",
  "auth.creating": "Criando...",
  "auth.noAccount": "Ainda não possui uma conta?",
  "auth.hasAccount": "Já possui uma conta?",
  "auth.loginIntro": "Entre para revisar projetos e análises.",
  "auth.registerIntro": "Cadastre-se para criar projetos e executar análises.",
  "auth.created": "Conta criada. Agora você pode entrar.",
  "auth.invalid": "Usuário ou senha inválidos.",
  "auth.tooManyLogin": "Muitas tentativas de login. Aguarde e tente novamente.",
  "auth.tooManyRegister": "Muitas tentativas de cadastro. Aguarde e tente novamente.",
  "auth.backendOffline": "O backend está offline ou inacessível. Inicie a API e tente novamente.",
  "auth.sessionExpired": "Sua sessão expirou. Entre novamente para continuar.",
  "profile.title": "Perfil",
  "profile.intro": "Informações da conta somente para leitura. O DataGuardian não expõe segredos nem tokens de autenticação aqui.",
  "profile.username": "Usuário",
  "profile.role": "Função",
  "profile.admin": "Administrador local",
  "profile.user": "Usuário",
  "profile.created": "Criado em",
  "profile.environment": "Ambiente",
  "profile.retention": "Retenção de órfãos",
  "profile.localPreferences": "Preferências locais",
  "profile.preferenceInfo": "As preferências de tema e idioma são armazenadas somente neste navegador.",
  "profile.loadError": "Não foi possível carregar o perfil.",
  "dashboard.profile": "Perfil",
  "dashboard.settings": "Configurações",
  "dashboard.darkMode": "Modo escuro",
  "dashboard.lightMode": "Modo claro",
  "dashboard.projects": "Projetos",
  "dashboard.analyses": "Análises",
  "dashboard.runAnalysis": "Executar análise",
  "dashboard.analysisHistory": "Histórico de análises",
  "dashboard.analysisDetails": "Detalhes da análise",
  "dashboard.findings": "Achados",
  "dashboard.metadata": "Metadados",
  "dashboard.safePreview": "Pré-visualização segura",
  "dashboard.originalFile": "Arquivo original",
  "dashboard.sanitizedFile": "Arquivo sanitizado",
  "dashboard.exportJson": "Exportar JSON",
  "dashboard.exportPdf": "Exportar PDF estático",
  "dashboard.copyLink": "Copiar link",
  "dashboard.linkCopied": "Link da análise copiado.",
  "dashboard.downloadOriginal": "Baixar original",
  "dashboard.downloadSanitized": "Baixar cópia sanitizada",
  "dashboard.analyzeFile": "Analisar arquivo",
  "dashboard.analyzeUrl": "Analisar URL",
  "dashboard.analyzingFile": "Analisando arquivo...",
  "dashboard.analyzingUrl": "Analisando URL...",
  "dashboard.risk": "Risco",
  "dashboard.status": "Status",
  "dashboard.type": "Tipo",
  "dashboard.clearFilters": "Limpar filtros",
  "dashboard.previous": "Anterior",
  "dashboard.next": "Próxima",
  "dashboard.view": "Visualizar",
  "dashboard.delete": "Excluir",
  "dashboard.storage": "Armazenamento",
  "dashboard.securityPreview": "Esta pré-visualização é estática e passiva. Conteúdo ativo do arquivo, JavaScript de sites e comportamento do navegador não são executados.",
  "dashboard.sanitizedWarning": "Esta é uma cópia separada com metadados removidos. Ela ainda pode conter conteúdo inseguro e não garante remoção de malware ou segurança completa.",
  "dashboard.originalWarning": "O arquivo original é preservado sem alterações. Revise a pré-visualização, os achados e o risco antes de baixar ou abrir localmente.",
};

type I18nContextValue = { locale: Locale; setLocale: (locale: Locale) => void; t: (key: string, fallback?: string) => string };
const I18nContext = createContext<I18nContextValue | null>(null);

export function I18nProvider({ children }: { children: ReactNode }) {
  const [locale, setLocaleState] = useState<Locale>("en");
  useEffect(() => {
    const saved = window.localStorage.getItem("dataguardian-locale");
    const initialLocale: Locale = saved === "pt-BR" ? "pt-BR" : "en";
    setLocaleState(initialLocale);
    document.documentElement.lang = initialLocale;
  }, []);
  function setLocale(next: Locale) {
    setLocaleState(next);
    window.localStorage.setItem("dataguardian-locale", next);
    document.documentElement.lang = next;
  }
  const t = (key: string, fallback = key) => locale === "pt-BR" ? (ptBR[key] ?? fallback) : fallback;
  return <I18nContext.Provider value={{ locale, setLocale, t }}>{children}</I18nContext.Provider>;
}

export function useI18n() {
  const value = useContext(I18nContext);
  if (!value) throw new Error("useI18n must be used inside I18nProvider");
  return value;
}

export function LanguageSwitcher() {
  const { locale, setLocale, t } = useI18n();
  return <label className="flex items-center gap-2 text-sm text-gray-700">
    <span>{t("language.label", "Language")}</span>
    <select aria-label={t("language.label", "Language")} className="rounded-md border border-gray-300 bg-white px-2 py-1 text-gray-900" value={locale} onChange={(event) => setLocale(event.target.value as Locale)}>
      <option value="en">{t("language.english", "English")}</option>
      <option value="pt-BR">{t("language.portuguese", "Português (Brasil)")}</option>
    </select>
  </label>;
}

import { defineConfig, type DefaultTheme } from 'vitepress'

// The portal is served from the root of its own domain, https://integra.adaraz.com.
const base = '/'

const GITHUB = 'https://github.com/ADARAZ-P-S-A/ADARAZ-INTEGRA'

// Each integration is its own section with its own sidebar, so nobody takes
// the pages of one for the other. The device page shows the three side by side.
function sidebar(prefix: string, t: Record<string, string>): DefaultTheme.Sidebar {
  const p = `${prefix}/raz-rcp-1`
  const back = { text: 'RAZ-RCP-1', items: [{ text: t.back, link: `${p}/` }] }
  const http = {
    text: t.http,
    items: [
      { text: t.overview, link: `${p}/http/` },
      { text: t.httpServer, link: `${p}/http/server` },
      { text: t.httpReader, link: `${p}/http/reader` },
      { text: t.httpProtocol, link: `${p}/http/protocol` },
      { text: t.httpTesting, link: `${p}/http/testing` },
    ],
  }
  const cloud = {
    text: t.cloud,
    items: [
      { text: t.overview, link: `${p}/cloud/` },
      { text: t.cloudRequirements, link: `${p}/cloud/requirements` },
      { text: t.cloudServer, link: `${p}/cloud/server` },
      { text: t.cloudPairing, link: `${p}/cloud/pairing` },
      { text: t.cloudUsers, link: `${p}/cloud/users` },
      { text: t.cloudPunches, link: `${p}/cloud/punches` },
      { text: t.cloudPresence, link: `${p}/cloud/presence` },
      { text: t.cloudResponses, link: `${p}/cloud/responses` },
    ],
  }
  const link = {
    text: t.link,
    items: [
      { text: t.overview, link: `${p}/link/` },
      { text: 'Home Assistant', link: `${p}/link/home-assistant` },
      { text: t.linkPython, link: `${p}/link/python` },
      { text: t.linkOther, link: `${p}/link/other-languages` },
      { text: t.linkSpec, link: `${p}/link/protocol` },
    ],
  }
  return {
    [`${p}/http/`]: [back, http],
    [`${p}/cloud/`]: [back, cloud],
    [`${p}/link/`]: [back, link],
    // The device page: one plain list, each integration opens its own section.
    [`${p}/`]: [
      {
        text: 'RAZ-RCP-1',
        items: [
          { text: t.choose, link: `${p}/` },
          { text: t.http, link: `${p}/http/` },
          { text: t.cloud, link: `${p}/cloud/` },
          { text: t.link, link: `${p}/link/` },
        ],
      },
    ],
  }
}

export default defineConfig({
  base,
  cleanUrls: true,
  sitemap: { hostname: 'https://integra.adaraz.com' },
  srcExclude: ['shared/**'],
  head: [['link', { rel: 'icon', type: 'image/svg+xml', href: `${base}adaraz-sygnet.svg` }]],

  themeConfig: {
    logo: { light: '/adaraz-sygnet.svg', dark: '/adaraz-sygnet-dark.svg' },
    socialLinks: [{ icon: 'github', link: GITHUB }],
    search: {
      provider: 'local',
      options: {
        locales: {
          root: {
            translations: {
              button: { buttonText: 'Szukaj', buttonAriaLabel: 'Szukaj' },
              modal: {
                noResultsText: 'Brak wyników dla',
                resetButtonTitle: 'Wyczyść',
                footer: { selectText: 'wybierz', navigateText: 'przejdź', closeText: 'zamknij' },
              },
            },
          },
        },
      },
    },
  },

  locales: {
    root: {
      label: 'Polski',
      lang: 'pl',
      title: 'Portal integratorów ADARAZ',
      description: 'Jak połączyć urządzenia ADARAZ z własnym systemem.',
      themeConfig: {
        nav: [
          { text: 'Urządzenia', link: '/' },
          { text: 'RAZ-RCP-1', link: '/raz-rcp-1/' },
        ],
        sidebar: sidebar('', {
          back: '← Wszystkie integracje',
          choose: 'Wybór integracji',
          overview: 'Przegląd',
          http: 'Wysyłanie odbić (HTTP)',
          httpServer: 'Serwer',
          httpReader: 'Ustawienia czytnika',
          httpProtocol: 'Protokół',
          httpTesting: 'Test i produkcja',
          cloud: 'Własny serwer chmury',
          cloudRequirements: 'Wymagania',
          cloudServer: 'Serwer wzorcowy',
          cloudPairing: 'Parowanie',
          cloudUsers: 'Lista pracowników',
          cloudPunches: 'Odbicia i nieznane karty',
          cloudPresence: 'Obecność',
          cloudResponses: 'Odpowiedzi i błędy',
          link: 'ADARAZ Link',
          linkPython: 'Własny serwer w Pythonie',
          linkOther: 'Inny język',
          linkSpec: 'Specyfikacja protokołu',
        }),
        outline: { label: 'Na tej stronie', level: [2, 3] },
        docFooter: { prev: 'Poprzednia', next: 'Następna' },
        langMenuLabel: 'Język',
        returnToTopLabel: 'Do góry',
        sidebarMenuLabel: 'Menu',
        darkModeSwitchLabel: 'Wygląd',
        lightModeSwitchTitle: 'Jasny motyw',
        darkModeSwitchTitle: 'Ciemny motyw',
        notFound: {
          title: 'NIE MA TAKIEJ STRONY',
          quote: 'Ta strona nie istnieje albo została przeniesiona.',
          linkText: 'Strona główna',
        },
        footer: { message: 'ADARAZ P.S.A.', copyright: '<a href="https://adaraz.com">adaraz.com</a>' },
      },
    },
    en: {
      label: 'English',
      lang: 'en',
      link: '/en/',
      title: 'ADARAZ integrator portal',
      description: 'How to connect ADARAZ devices to your own system.',
      themeConfig: {
        nav: [
          { text: 'Devices', link: '/en/' },
          { text: 'RAZ-RCP-1', link: '/en/raz-rcp-1/' },
        ],
        sidebar: sidebar('/en', {
          back: '← All integrations',
          choose: 'Choose an integration',
          overview: 'Overview',
          http: 'Punch push (HTTP)',
          httpServer: 'The server',
          httpReader: 'Reader setup',
          httpProtocol: 'Protocol',
          httpTesting: 'Testing and production',
          cloud: 'Own cloud server',
          cloudRequirements: 'Requirements',
          cloudServer: 'Reference server',
          cloudPairing: 'Pairing',
          cloudUsers: 'Employee list',
          cloudPunches: 'Punches and unknown cards',
          cloudPresence: 'Presence',
          cloudResponses: 'Responses and errors',
          link: 'ADARAZ Link',
          linkPython: 'Your own server in Python',
          linkOther: 'Another language',
          linkSpec: 'Protocol specification',
        }),
        outline: { level: [2, 3] },
        footer: { message: 'ADARAZ P.S.A.', copyright: '<a href="https://adaraz.com">adaraz.com</a>' },
      },
    },
  },
})

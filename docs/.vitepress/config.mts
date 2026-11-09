import { defineConfig, type DefaultTheme } from 'vitepress'

// The portal is served from the root of its own domain, https://integra.adaraz.com.
const base = '/'

const GITHUB = 'https://github.com/ADARAZ-P-S-A'

function sidebar(prefix: string, t: Record<string, string>): DefaultTheme.Sidebar {
  return {
    [`${prefix}/raz-rcp-1/`]: [
      {
        text: 'RAZ-RCP-1',
        items: [
          { text: t.overview, link: `${prefix}/raz-rcp-1/` },
          { text: t.http, link: `${prefix}/raz-rcp-1/http` },
          {
            text: t.cloud,
            link: `${prefix}/raz-rcp-1/cloud/`,
            collapsed: false,
            items: [
              { text: t.cloudRequirements, link: `${prefix}/raz-rcp-1/cloud/requirements` },
              { text: t.cloudServer, link: `${prefix}/raz-rcp-1/cloud/server` },
              { text: t.cloudPairing, link: `${prefix}/raz-rcp-1/cloud/pairing` },
              { text: t.cloudUsers, link: `${prefix}/raz-rcp-1/cloud/users` },
              { text: t.cloudPunches, link: `${prefix}/raz-rcp-1/cloud/punches` },
              { text: t.cloudPresence, link: `${prefix}/raz-rcp-1/cloud/presence` },
              { text: t.cloudResponses, link: `${prefix}/raz-rcp-1/cloud/responses` },
            ],
          },
          { text: t.link, link: `${prefix}/raz-rcp-1/link` },
          { text: t.spec, link: `${prefix}/raz-rcp-1/link-protocol` },
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
          overview: 'Wybór integracji',
          http: 'Wysyłanie odbić (HTTP)',
          cloud: 'Własny serwer chmury',
          cloudRequirements: 'Wymagania',
          cloudServer: 'Serwer wzorcowy',
          cloudPairing: 'Parowanie',
          cloudUsers: 'Lista pracowników',
          cloudPunches: 'Odbicia i nieznane karty',
          cloudPresence: 'Obecność',
          cloudResponses: 'Odpowiedzi i błędy',
          link: 'ADARAZ Link',
          spec: 'Specyfikacja ADARAZ Link',
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
          overview: 'Choose an integration',
          http: 'Punch push (HTTP)',
          cloud: 'Own cloud server',
          cloudRequirements: 'Requirements',
          cloudServer: 'Reference server',
          cloudPairing: 'Pairing',
          cloudUsers: 'Employee list',
          cloudPunches: 'Punches and unknown cards',
          cloudPresence: 'Presence',
          cloudResponses: 'Responses and errors',
          link: 'ADARAZ Link',
          spec: 'ADARAZ Link specification',
        }),
        outline: { level: [2, 3] },
        footer: { message: 'ADARAZ P.S.A.', copyright: '<a href="https://adaraz.com">adaraz.com</a>' },
      },
    },
  },
})

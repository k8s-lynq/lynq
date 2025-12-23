import { Outlet, Link, useLocation } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import {
  IconLayoutDashboard,
  IconDatabase,
  IconFileCode,
  IconBox,
  IconMoon,
  IconSun,
  IconSearch,
  IconSitemap,
  IconBook,
  IconBrandGithub,
  IconLanguage,
  IconCheck,
} from '@tabler/icons-react'
import { Button } from '@/components/ui/button'
import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbLink,
  BreadcrumbList,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from '@/components/ui/breadcrumb'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { useTheme } from '@/components/theme-provider'
import { SearchCommand } from '@/components/SearchCommand'
import { cn } from '@/lib/utils'
import { supportedLanguages } from '@/i18n'

const navigation = [
  { nameKey: 'nav.overview', href: '/', icon: IconLayoutDashboard, matchExact: true },
  { nameKey: 'nav.topology', href: '/topology', icon: IconSitemap },
  { nameKey: 'nav.hubs', href: '/hubs', icon: IconDatabase },
  { nameKey: 'nav.forms', href: '/forms', icon: IconFileCode },
  { nameKey: 'nav.nodes', href: '/nodes', icon: IconBox },
]

// External links - can be hidden via environment variables (opt-out)
const DOCS_URL = import.meta.env.VITE_DOCS_URL || 'https://lynq.sh/'
const GITHUB_URL = import.meta.env.VITE_GITHUB_URL || 'https://github.com/k8s-lynq/lynq'
const SHOW_DOCS_LINK = import.meta.env.VITE_HIDE_DOCS_LINK !== 'true'
const SHOW_GITHUB_LINK = import.meta.env.VITE_HIDE_GITHUB_LINK !== 'true'

// Map route segments to i18n keys
const routeLabelKeys: Record<string, string> = {
  '': 'nav.overview',
  'topology': 'nav.topology',
  'hubs': 'nav.hubs',
  'forms': 'nav.forms',
  'nodes': 'nav.nodes',
}

function useBreadcrumbs() {
  const location = useLocation()
  const { t } = useTranslation()

  const pathSegments = location.pathname.split('/').filter(Boolean)

  const breadcrumbs: { label: string; href?: string }[] = []

  if (pathSegments.length === 0) {
    // On root page, no breadcrumbs needed
    return []
  }

  // First segment is the resource type (hubs, forms, nodes, topology)
  const resourceType = pathSegments[0]
  const labelKey = routeLabelKeys[resourceType]
  const resourceLabel = labelKey ? t(labelKey) : resourceType

  if (pathSegments.length === 1) {
    // On list page (e.g., /hubs), just show the resource type as current page
    breadcrumbs.push({ label: resourceLabel })
  } else if (pathSegments.length >= 2) {
    // On detail page (e.g., /hubs/my-hub)
    breadcrumbs.push({ label: resourceLabel, href: `/${resourceType}` })

    // Add the resource name as the current page
    const resourceName = decodeURIComponent(pathSegments[1])
    breadcrumbs.push({ label: resourceName })
  }

  return breadcrumbs
}

export function Layout() {
  const { theme, setTheme } = useTheme()
  const { t, i18n } = useTranslation()
  const location = useLocation()
  const breadcrumbs = useBreadcrumbs()

  // Helper to check if a nav item is active
  const isNavActive = (item: typeof navigation[0]) => {
    if (item.matchExact) {
      return location.pathname === item.href
    }
    // For non-exact matches, check if the path starts with the href
    // but make sure we don't match / with everything
    if (item.href === '/') {
      return location.pathname === '/'
    }
    return location.pathname.startsWith(item.href)
  }

  // Get current section name for header
  const getCurrentSectionName = () => {
    const activeNav = navigation.find(item => isNavActive(item))
    return activeNav ? t(activeNav.nameKey) : t('nav.dashboard')
  }

  // Handle language change
  const handleLanguageChange = (langCode: string) => {
    i18n.changeLanguage(langCode)
  }

  return (
    <div className="flex h-screen bg-background">
      {/* Sidebar */}
      <aside className="w-64 border-r bg-card flex flex-col">
        <div className="flex h-16 items-center border-b px-6">
          <Link to="/" className="flex items-center gap-2">
            <img src="/logo.png" alt="Lynq" className="h-6 w-6" />
            <span className="text-xl font-bold">Lynq</span>
          </Link>
        </div>
        <nav className="flex flex-col gap-1 p-4 flex-1">
          {navigation.map((item) => {
            const isActive = isNavActive(item)
            return (
              <Link
                key={item.nameKey}
                to={item.href}
                className={cn(
                  'flex items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium transition-colors',
                  isActive
                    ? 'bg-primary text-primary-foreground font-semibold'
                    : 'text-muted-foreground hover:bg-accent hover:text-accent-foreground'
                )}
              >
                <item.icon size={16} />
                {t(item.nameKey)}
              </Link>
            )
          })}
        </nav>

        {/* External Links */}
        {(SHOW_DOCS_LINK || SHOW_GITHUB_LINK) && (
          <div className="border-t p-4">
            <div className="flex items-center gap-2">
              {SHOW_DOCS_LINK && (
                <Tooltip>
                  <TooltipTrigger asChild>
                    <a
                      href={DOCS_URL}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="flex items-center justify-center w-8 h-8 rounded-lg text-muted-foreground hover:bg-accent hover:text-accent-foreground transition-colors"
                    >
                      <IconBook size={18} />
                    </a>
                  </TooltipTrigger>
                  <TooltipContent side="top">{t('common.documentation')}</TooltipContent>
                </Tooltip>
              )}
              {SHOW_GITHUB_LINK && (
                <Tooltip>
                  <TooltipTrigger asChild>
                    <a
                      href={GITHUB_URL}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="flex items-center justify-center w-8 h-8 rounded-lg text-muted-foreground hover:bg-accent hover:text-accent-foreground transition-colors"
                    >
                      <IconBrandGithub size={18} />
                    </a>
                  </TooltipTrigger>
                  <TooltipContent side="top">{t('common.github')}</TooltipContent>
                </Tooltip>
              )}
            </div>
          </div>
        )}
      </aside>

      {/* Main content */}
      <div className="flex flex-1 flex-col overflow-hidden">
        {/* Header */}
        <header className="flex h-16 items-center justify-between border-b px-6">
          <div className="flex items-center gap-4">
            {breadcrumbs.length > 0 ? (
              <Breadcrumb>
                <BreadcrumbList>
                  <BreadcrumbItem>
                    <Tooltip>
                      <TooltipTrigger asChild>
                        <BreadcrumbLink asChild>
                          <Link to="/" className="cursor-pointer">
                            <IconLayoutDashboard size={14} className="mr-1" />
                          </Link>
                        </BreadcrumbLink>
                      </TooltipTrigger>
                      <TooltipContent>{t('common.home')}</TooltipContent>
                    </Tooltip>
                  </BreadcrumbItem>
                  {breadcrumbs.map((crumb, index) => (
                    <span key={index} className="contents">
                      <BreadcrumbSeparator />
                      <BreadcrumbItem>
                        {crumb.href ? (
                          <BreadcrumbLink asChild>
                            <Link to={crumb.href}>{crumb.label}</Link>
                          </BreadcrumbLink>
                        ) : (
                          <BreadcrumbPage>{crumb.label}</BreadcrumbPage>
                        )}
                      </BreadcrumbItem>
                    </span>
                  ))}
                </BreadcrumbList>
              </Breadcrumb>
            ) : (
              <h1 className="text-lg font-semibold">{getCurrentSectionName()}</h1>
            )}
          </div>
          <div className="flex items-center gap-2">
            <Button
              variant="outline"
              size="sm"
              className="w-64 justify-start text-muted-foreground"
              onClick={() => {
                // Trigger the search dialog via keyboard event
                const event = new KeyboardEvent('keydown', {
                  key: 'k',
                  metaKey: true,
                  bubbles: true,
                })
                document.dispatchEvent(event)
              }}
            >
              <IconSearch size={16} className="mr-2" />
              <span className="flex-1 text-left">{t('search.placeholder')}</span>
              <kbd className="pointer-events-none ml-auto inline-flex h-5 select-none items-center gap-1 rounded border bg-muted px-1.5 font-mono text-[10px] font-medium text-muted-foreground">
                <span className="text-xs">⌘</span>K
              </kbd>
            </Button>
            {/* Language Switcher */}
            <DropdownMenu>
              <Tooltip>
                <TooltipTrigger asChild>
                  <DropdownMenuTrigger asChild>
                    <Button variant="ghost" size="icon">
                      <IconLanguage size={16} />
                    </Button>
                  </DropdownMenuTrigger>
                </TooltipTrigger>
                <TooltipContent>{t('common.language')}</TooltipContent>
              </Tooltip>
              <DropdownMenuContent align="end">
                {supportedLanguages.map((lang) => (
                  <DropdownMenuItem
                    key={lang.code}
                    onClick={() => handleLanguageChange(lang.code)}
                    className="flex items-center justify-between"
                  >
                    <span>{lang.nativeName}</span>
                    {i18n.language === lang.code && (
                      <IconCheck size={14} className="text-primary" />
                    )}
                  </DropdownMenuItem>
                ))}
              </DropdownMenuContent>
            </DropdownMenu>
            <Tooltip>
              <TooltipTrigger asChild>
                <Button
                  variant="ghost"
                  size="icon"
                  onClick={() => setTheme(theme === 'dark' ? 'light' : 'dark')}
                >
                  {theme === 'dark' ? (
                    <IconSun size={16} />
                  ) : (
                    <IconMoon size={16} />
                  )}
                </Button>
              </TooltipTrigger>
              <TooltipContent>
                {theme === 'dark' ? t('theme.switchToLight') : t('theme.switchToDark')}
              </TooltipContent>
            </Tooltip>
          </div>
        </header>
        <SearchCommand />

        {/* Page content */}
        <main className="flex-1 overflow-auto p-6">
          <Outlet />
        </main>
      </div>
    </div>
  )
}

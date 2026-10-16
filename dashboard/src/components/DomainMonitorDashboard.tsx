import { useState, useEffect, useCallback } from 'react'
import { Card, CardContent, CardHeader, CardTitle } from '@/ui/card'
import { Badge } from '@/ui/badge'
import { Button } from '@/ui/button'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/ui/dropdown-menu'
import {
  RefreshCw,
  Activity,
  AlertTriangle,
  CheckCircle,
  XCircle,
  Clock,
  Search,
  Play,
  ChevronDown,
  Sun,
  Moon,
  Monitor,
  Link2,
  Cloud,
  Layers,
  Radio,
  CirclePause,
  type LucideIcon,
} from 'lucide-react'
import { cn } from '@/lib/utils'
import { useTheme, type Theme } from '@/lib/theme'
import { apiFetch, pageBuildInfo, type BuildInfo } from '@/lib/build'
import { setFavicon } from '@/lib/favicon'
import type { OverallStatus } from '@/lib/favicon-shapes'
import { StatusIcon } from '@/components/StatusIcon'
import { EndpointHistory } from '@/components/EndpointHistory'

interface DomainStatusEndpoint {
  // Name of the endpoint, if it has one
  name?: string
  healthy: boolean
  // IP address, or CNAME hostname
  ip: string
  // Record type: A, AAAA or CNAME
  type?: string
  proxied?: boolean
  // Lower values are preferred
  priority?: number
  // True if published in DNS
  active?: boolean
  failureCount?: number
  // When the endpoint last changed state, as an RFC 3339 time
  since?: string
  // True if `since` is when ddup first checked the endpoint
  sinceStart?: boolean
  // Last time a check of the endpoint failed
  lastFailure?: string
  // Error of the last failed check
  lastError?: string
  // First failed check of the current run of failures
  firstFailure?: string
}

interface DomainStatus {
  lastUpdated: string
  provider: string
  error?: string
  endpoints: DomainStatusEndpoint[]
}

type DomainsResponse = Record<string, DomainStatus>

type Domain = {
  name: string
  status: DomainStatus
}

// True if the endpoints have more than one priority, so it matters which of them are published
const hasTiers = (status: DomainStatus): boolean => new Set(status.endpoints.map((e) => e.priority ?? 0)).size > 1

// Shortens long values (like tunnel hostnames) in the middle, so the end, which says what kind of host it is, stays visible
const shortenMiddle = (value: string, max = 26): string => {
  if (value.length <= max) {
    return value
  }
  const tail = Math.min(17, Math.ceil((max - 1) * 0.65))
  const head = max - 1 - tail
  return `${value.slice(0, head)}…${value.slice(value.length - tail)}`
}

// Orders endpoints by priority, then the ones published in DNS first, then healthy ones, then by name
const sortEndpoints = (endpoints: DomainStatusEndpoint[]): DomainStatusEndpoint[] =>
  [...endpoints].sort(
    (a, b) =>
      (a.priority ?? 0) - (b.priority ?? 0) ||
      Number(!!b.active) - Number(!!a.active) ||
      Number(b.healthy) - Number(a.healthy) ||
      (a.name ?? a.ip).localeCompare(b.name ?? b.ip)
  )

// A small icon (with an optional label) that explains itself on hover and to screen readers
// Every state has its own shape and a text, so none relies on color alone
const Chip = ({
  icon: Icon,
  label,
  title,
  className,
}: {
  icon: LucideIcon
  label?: string
  title: string
  className?: string
}) => (
  <span
    title={title}
    className={cn(
      'inline-flex items-center gap-1 rounded-md border px-1.5 py-0.5 text-xs font-medium text-muted-foreground',
      className
    )}
  >
    <Icon className="h-3.5 w-3.5" aria-hidden="true" />
    {label && <span aria-hidden="true">{label}</span>}
    <span className="sr-only">{title}</span>
  </span>
)

// Classes of the chip of an endpoint that is published in DNS
const publishedChipClass = 'border-green-600/40 text-green-600 dark:border-green-400/40 dark:text-green-400'

// One segment of the summary bar: the count of domains with a health status
// The label is only shown from the sm breakpoint up, and stays available to screen readers
const SummarySegment = ({
  icon: Icon,
  label,
  title,
  count,
  className,
}: {
  icon: LucideIcon
  label: string
  title: string
  count: number
  className: string
}) => (
  <div
    title={title}
    className={cn(
      'flex items-center justify-center gap-1.5 px-1 py-1.5 sm:gap-2.5 sm:p-2.5 lg:gap-3 lg:p-3.5',
      className
    )}
  >
    <Icon className="h-4 w-4 shrink-0 sm:h-5 sm:w-5 lg:h-6 lg:w-6" aria-hidden="true" />
    <span className="text-base leading-none font-bold sm:text-2xl lg:text-3xl">{count}</span>
    <span className="sr-only text-[13px] font-medium text-muted-foreground sm:not-sr-only lg:text-sm">{label}</span>
  </div>
)

const themeOrder: Theme[] = ['system', 'light', 'dark']
const themeIcons: Record<Theme, LucideIcon> = { system: Monitor, light: Sun, dark: Moon }

// Icon button that cycles the theme: follow the system, light, dark
// The label says the current theme and the next, so it does not rely on the icon alone
const ThemeSwitcher = () => {
  const { theme, setTheme } = useTheme()
  const next = themeOrder[(themeOrder.indexOf(theme) + 1) % themeOrder.length]
  const Icon = themeIcons[theme]
  const label = `Theme: ${theme} (click for ${next})`

  return (
    <Button variant="outline" size="icon" onClick={() => setTheme(next)} title={label} aria-label={label}>
      <Icon className="h-4 w-4" aria-hidden="true" />
    </Button>
  )
}

// Explains the icons used on the endpoints
const Legend = () => (
  <div className="flex flex-wrap items-center gap-x-4 gap-y-2 text-xs text-muted-foreground">
    <span className="inline-flex items-center gap-1.5">
      <Chip icon={Link2} title="CNAME record" /> CNAME record
    </span>
    <span className="inline-flex items-center gap-1.5">
      <Chip icon={Cloud} title="Proxied by Cloudflare" className="text-orange-600 dark:text-orange-400" /> Proxied by
      Cloudflare
    </span>
    <span className="inline-flex items-center gap-1.5">
      <Chip icon={Layers} label="n" title="Priority" /> Priority n (lower is preferred)
    </span>
    <span className="inline-flex items-center gap-1.5">
      <Chip icon={Radio} title="Published in DNS" className={publishedChipClass} /> Published in DNS
    </span>
    <span className="inline-flex items-center gap-1.5">
      <Chip icon={CirclePause} title="Healthy, on standby" /> Healthy, on standby
    </span>
    <span className="inline-flex items-center gap-1.5">
      <Chip icon={AlertTriangle} label="n" title="Failed checks" className="text-yellow-700 dark:text-yellow-400" />{' '}
      Consecutive failed checks
    </span>
  </div>
)

const EndpointRow = ({ endpoint, tiered }: { endpoint: DomainStatusEndpoint; tiered: boolean }) => {
  const StateIcon = endpoint.healthy ? CheckCircle : XCircle
  const state = endpoint.healthy ? 'Healthy' : 'Unhealthy'
  const failures = endpoint.failureCount ?? 0
  // The name is the main label when there is one, and the address is secondary
  const primary = endpoint.name || endpoint.ip
  const showTarget = !!endpoint.name

  return (
    <div className="flex flex-wrap items-center gap-x-3 gap-y-1.5 rounded-lg border p-2">
      <div className="flex min-w-0 grow basis-48 items-center gap-2">
        <span title={state} className="shrink-0">
          <StateIcon
            className={cn(
              'h-5 w-5',
              endpoint.healthy ? 'text-green-600 dark:text-green-400' : 'text-red-600 dark:text-red-400'
            )}
            aria-hidden="true"
          />
          <span className="sr-only">{state}</span>
        </span>
        <div className="min-w-0">
          <div className={cn('truncate text-sm', showTarget ? 'font-medium' : 'font-mono')} title={primary}>
            {primary}
          </div>
          {showTarget && (
            <div className="truncate font-mono text-xs text-muted-foreground" title={endpoint.ip}>
              {shortenMiddle(endpoint.ip)}
            </div>
          )}
        </div>
      </div>

      {/* Always on the right: next to the name when it fits, and on its own line, still on the right, when it does not */}
      <div className="ml-auto flex shrink-0 flex-wrap items-center justify-end gap-1.5">
        {endpoint.type === 'CNAME' && <Chip icon={Link2} title="CNAME record" />}
        {endpoint.proxied && (
          <Chip icon={Cloud} title="Proxied by Cloudflare" className="text-orange-600 dark:text-orange-400" />
        )}
        {tiered && (
          <Chip icon={Layers} label={String(endpoint.priority ?? 0)} title={`Priority ${endpoint.priority ?? 0}`} />
        )}
        {tiered && endpoint.healthy && (
          <Chip
            icon={endpoint.active ? Radio : CirclePause}
            title={endpoint.active ? 'Published in DNS' : 'Healthy, on standby'}
            className={endpoint.active ? publishedChipClass : undefined}
          />
        )}
        {failures > 0 && (
          <Chip
            icon={AlertTriangle}
            label={String(failures)}
            title={`${failures} consecutive failed ${failures === 1 ? 'check' : 'checks'}`}
            className="text-yellow-700 dark:text-yellow-400"
          />
        )}
      </div>
    </div>
  )
}

// The build that served the page, at the bottom of the page
// The commit is left out when the build doesn't set it (a development build)
const Footer = ({ info }: { info: BuildInfo }) => {
  const parts = [`ddup ${info.version}`, info.commit ? info.commit.slice(0, 7) : null].filter(Boolean)

  return <footer className="pt-2 text-center text-xs wrap-break-word text-muted-foreground">{parts.join(' · ')}</footer>
}

const DomainMonitorDashboard = ({ endpoint }: { endpoint: string }) => {
  const [domains, setDomains] = useState<Domain[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [lastUpdated, setLastUpdated] = useState<Date | null>(null)
  const [autoRefresh, setAutoRefresh] = useState(true)
  const [searchTerm, setSearchTerm] = useState('')
  const [isChecking, setIsChecking] = useState(false)
  // The domain whose endpoint history is open; only one can be at a time
  const [historyDomain, setHistoryDomain] = useState<string | null>(null)

  const fetchDomains = useCallback(async (): Promise<void> => {
    setIsLoading(true)
    setError(null) // Clear previous errors
    try {
      const response = await apiFetch(endpoint + '/api/status')
      if (!response.ok) {
        throw new Error(`HTTP error: ${response.status} ${response.statusText}`)
      }
      const data: DomainsResponse = await response.json()

      // Convert the response format to our Domain array
      const domainsArray: Domain[] = Object.entries(data).map(([name, status]) => ({
        name,
        status,
      }))

      setDomains(domainsArray)
      setLastUpdated(new Date())
    } catch (error) {
      console.error('Failed to fetch domains data:', error)
      const errorMessage = error instanceof Error ? error.message : 'Unknown error occurred'
      setError(`Failed to fetch domain data: ${errorMessage}`)

      // Fallback to empty array on error
      setDomains([])
    } finally {
      setIsLoading(false)
    }
  }, [endpoint])

  useEffect(() => {
    if (!autoRefresh) {
      return
    }

    const controller = new AbortController()

    const runFetch = async () => {
      try {
        setIsLoading(true)
        setError(null)

        const response = await apiFetch(endpoint + '/api/status', { signal: controller.signal })
        if (!response.ok) {
          throw new Error(`HTTP error: ${response.status} ${response.statusText}`)
        }

        const data: DomainsResponse = await response.json()
        if (controller.signal.aborted) {
          return
        }

        const domainsArray: Domain[] = Object.entries(data).map(([name, status]) => ({ name, status }))
        setDomains(domainsArray)
        setLastUpdated(new Date())
      } catch (err) {
        if (controller.signal.aborted || (err instanceof DOMException && err.name === 'AbortError')) {
          return
        }

        console.error('Failed to fetch domains data:', err)

        const errorMessage = err instanceof Error ? err.message : 'Unknown error occurred'
        setError(`Failed to fetch domain data: ${errorMessage}`)
        setDomains([])
      } finally {
        if (!controller.signal.aborted) {
          setIsLoading(false)
        }
      }
    }

    // Defer the initial fetch so the effect body itself does not call setState synchronously.
    queueMicrotask(() => {
      void runFetch()
    })

    const interval = setInterval(() => {
      if (autoRefresh) {
        void runFetch()
      }
    }, 60000)

    return () => {
      controller.abort()
      clearInterval(interval)
    }
  }, [autoRefresh, endpoint])

  const refreshClicked = async () => {
    await fetchDomains()
  }

  // Asks the server to run health checks right now (instead of waiting for the next interval), then shows the result
  const checkNowClicked = async (): Promise<void> => {
    setIsChecking(true)
    setError(null)
    try {
      const response = await apiFetch(endpoint + '/api/check', {
        method: 'POST',
        // The server requires this header, which blocks cross-site requests
        headers: { 'X-Requested-By': 'ddup-dashboard' },
      })
      if (!response.ok) {
        throw new Error(`HTTP error: ${response.status} ${response.statusText}`)
      }

      // The response includes the updated status
      const data: DomainsResponse = await response.json()
      setDomains(Object.entries(data).map(([name, status]) => ({ name, status })))
      setLastUpdated(new Date())
    } catch (err) {
      console.error('Failed to run check:', err)
      const errorMessage = err instanceof Error ? err.message : 'Unknown error occurred'
      setError(`Failed to run check: ${errorMessage}`)
    } finally {
      setIsChecking(false)
    }
  }

  const getDomainStatus = (domain: Domain) => {
    if (domain.status.error) {
      return 'unhealthy'
    }
    if (domain.status.endpoints.length === 0) {
      return 'unhealthy'
    }

    const healthyEndpoints = domain.status.endpoints.filter((e) => e.healthy).length
    const totalEndpoints = domain.status.endpoints.length

    if (healthyEndpoints === totalEndpoints) {
      return 'healthy'
    }
    if (healthyEndpoints > 0) {
      return 'warning'
    }
    return 'unhealthy'
  }

  const filteredDomains = domains.filter((domain) => {
    const searchLower = searchTerm.toLowerCase()
    const domainMatches = domain.name.toLowerCase().includes(searchLower)
    const endpointMatches = domain.status.endpoints.some(
      (endpoint) =>
        endpoint.ip.toLowerCase().includes(searchLower) || (endpoint.name ?? '').toLowerCase().includes(searchLower)
    )
    return domainMatches || endpointMatches
  })

  // The icon of the tab shows the worst status of all the domains (not only the ones matching the search)
  const overallCounts = {
    warning: domains.filter((d) => getDomainStatus(d) === 'warning').length,
    unhealthy: domains.filter((d) => getDomainStatus(d) === 'unhealthy').length,
  }
  let overallStatus: OverallStatus = 'unknown'
  if (error || overallCounts.unhealthy > 0) {
    overallStatus = 'unhealthy'
  } else if (overallCounts.warning > 0) {
    overallStatus = 'warning'
  } else if (domains.length > 0) {
    overallStatus = 'healthy'
  }
  useEffect(() => {
    setFavicon(overallStatus)
  }, [overallStatus])

  const healthyDomains = filteredDomains.filter((d) => getDomainStatus(d) === 'healthy').length
  const warningDomains = filteredDomains.filter((d) => getDomainStatus(d) === 'warning').length
  const unhealthyDomains = filteredDomains.filter((d) => getDomainStatus(d) === 'unhealthy').length

  return (
    <div className="min-h-screen bg-background p-4 md:p-6">
      <div className="mx-auto max-w-7xl space-y-6">
        {/* Header */}
        <div className="flex flex-col gap-4 md:flex-row md:items-center md:justify-between">
          <div className="flex items-center gap-3">
            <StatusIcon status={overallStatus} className="h-8 w-8 shrink-0" />
            <h1 className="text-3xl font-bold tracking-tight">ddup</h1>
          </div>

          <div className="flex flex-col gap-2 sm:flex-row sm:items-center">
            {lastUpdated && (
              <div className="flex items-center gap-2 text-sm text-muted-foreground">
                <Clock className="h-4 w-4" />
                Last updated: {lastUpdated.toLocaleTimeString()}
              </div>
            )}

            <div className="flex flex-wrap gap-2">
              {/* Split button: the main part toggles auto-refresh, the chevron opens a menu to refresh right away */}
              <div className="flex">
                <Button
                  variant={autoRefresh ? 'default' : 'outline'}
                  size="sm"
                  onClick={() => setAutoRefresh(!autoRefresh)}
                  className="flex items-center gap-2 rounded-r-none"
                >
                  <Activity className="h-4 w-4" />
                  Auto-refresh {autoRefresh ? 'ON' : 'OFF'}
                </Button>

                <DropdownMenu>
                  <DropdownMenuTrigger asChild>
                    <Button
                      variant={autoRefresh ? 'default' : 'outline'}
                      size="sm"
                      aria-label="More refresh options"
                      className={cn(
                        'rounded-l-none',
                        autoRefresh ? 'border-l border-primary-foreground/20' : 'border-l-0'
                      )}
                    >
                      <ChevronDown className="h-4 w-4" />
                    </Button>
                  </DropdownMenuTrigger>
                  <DropdownMenuContent align="end">
                    <DropdownMenuItem onSelect={refreshClicked} disabled={isLoading}>
                      <RefreshCw className="h-4 w-4" />
                      Refresh now
                    </DropdownMenuItem>
                  </DropdownMenuContent>
                </DropdownMenu>
              </div>

              <Button
                variant="outline"
                size="sm"
                onClick={checkNowClicked}
                disabled={isChecking || isLoading}
                title="Run health checks now, instead of waiting for the next interval"
                className="flex items-center gap-2"
              >
                <Play className={cn('h-4 w-4', isChecking && 'animate-pulse')} />
                {isChecking ? 'Checking…' : 'Check now'}
              </Button>

              <ThemeSwitcher />
            </div>
          </div>
        </div>

        {/* Search bar */}
        <div className="relative w-full max-w-md">
          <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
          <input
            type="text"
            placeholder="Search domains, endpoints or IP addresses..."
            value={searchTerm}
            onChange={(e) => {
              setSearchTerm(e.target.value)
              setHistoryDomain(null)
            }}
            className="w-full rounded-md border border-input bg-background px-10 py-2 text-sm ring-offset-background placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
          />
        </div>

        {/* Summary of the domains: three equal segments in one bar */}
        {!error && domains.length > 0 && (
          <div className="grid grid-cols-3 divide-x overflow-hidden rounded-md border bg-card">
            <SummarySegment
              icon={CheckCircle}
              label="Healthy"
              title="Healthy domains"
              count={healthyDomains}
              className="text-green-600 dark:text-green-400"
            />
            <SummarySegment
              icon={AlertTriangle}
              label="Warning"
              title="Domains with some unhealthy endpoints"
              count={warningDomains}
              className="text-yellow-600 dark:text-yellow-400"
            />
            <SummarySegment
              icon={XCircle}
              label="Unhealthy"
              title="Unhealthy domains"
              count={unhealthyDomains}
              className="text-red-600 dark:text-red-400"
            />
          </div>
        )}

        {/* Error Message */}
        {error && (
          <Card className="border-red-200 bg-red-50 dark:border-red-800 dark:bg-red-950/50 lg:w-3/5 mx-auto">
            <CardContent>
              <div className="flex items-center gap-2 text-red-800 dark:text-red-200">
                <XCircle className="h-5 w-5" />
                <span className="font-medium">Error</span>
              </div>
              <p className="mt-2 text-sm text-red-700 dark:text-red-300">{error}</p>
            </CardContent>
          </Card>
        )}

        {/* Domain Cards */}
        {!error && domains.length > 0 && (
          <div className="grid gap-6 md:grid-cols-2 lg:grid-cols-3">
            {filteredDomains.map((domain) => {
              const status = getDomainStatus(domain)
              return (
                <Card key={domain.name} className="overflow-hidden">
                  <CardHeader className="pb-2">
                    <div className="flex items-center justify-between">
                      <CardTitle className="text-lg">{domain.name}</CardTitle>
                      <Badge
                        variant={status === 'healthy' ? 'default' : status === 'warning' ? 'secondary' : 'destructive'}
                        className={cn(
                          'flex items-center gap-1',
                          status === 'healthy' &&
                            'bg-green-100 text-green-800 hover:bg-green-100 dark:bg-green-900/30 dark:text-green-400 dark:hover:bg-green-900/30',
                          status === 'warning' &&
                            'bg-yellow-100 text-yellow-800 hover:bg-yellow-100 dark:bg-yellow-900/30 dark:text-yellow-400 dark:hover:bg-yellow-900/30',
                          status === 'unhealthy' &&
                            'bg-red-100 text-red-800 hover:bg-red-100 dark:bg-red-900/30 dark:text-red-400 dark:hover:bg-red-900/30'
                        )}
                      >
                        {status === 'healthy' && <CheckCircle className="h-3 w-3" />}
                        {status === 'warning' && <AlertTriangle className="h-3 w-3" />}
                        {status === 'unhealthy' && <XCircle className="h-3 w-3" />}
                        {status.charAt(0).toUpperCase() + status.slice(1)}
                      </Badge>
                    </div>
                    <div className="text-xs text-muted-foreground">Provider: {domain.status.provider}</div>
                    {/* The history is about the endpoints, so its button is on the line that counts them */}
                    <div className="flex flex-wrap items-center justify-between gap-x-2 text-sm text-muted-foreground">
                      <span>
                        {domain.status.endpoints.filter((e) => e.healthy).length}/{domain.status.endpoints.length}{' '}
                        endpoints healthy
                      </span>
                      <EndpointHistory
                        domain={domain.name}
                        lastUpdated={domain.status.lastUpdated}
                        endpoints={domain.status.endpoints}
                        open={historyDomain === domain.name}
                        onOpenChange={(open) => setHistoryDomain(open ? domain.name : null)}
                      />
                    </div>
                  </CardHeader>

                  <CardContent className="space-y-4">
                    {domain.status.error && (
                      <div className="rounded-lg bg-red-50 dark:bg-red-950/50 p-3 text-sm text-red-800 dark:text-red-200">
                        <div className="flex items-center gap-2">
                          <XCircle className="h-4 w-4" />
                          <span className="font-medium">Domain Error</span>
                        </div>
                        <p className="mt-1">{domain.status.error}</p>
                      </div>
                    )}

                    {/* Endpoints */}
                    {domain.status.endpoints.length > 0 && (
                      <div>
                        <h4 className="mb-2 text-sm font-medium">Endpoints ({domain.status.endpoints.length})</h4>
                        <div className="space-y-2">
                          {sortEndpoints(domain.status.endpoints).map((endpoint) => (
                            <EndpointRow
                              key={`${endpoint.name ?? ''}|${endpoint.ip}`}
                              endpoint={endpoint}
                              tiered={hasTiers(domain.status)}
                            />
                          ))}
                        </div>
                      </div>
                    )}
                  </CardContent>
                </Card>
              )
            })}
          </div>
        )}

        {!error && domains.length > 0 && <Legend />}

        {/* No Results Message */}
        {filteredDomains.length === 0 && searchTerm && !isLoading && (
          <div className="text-center py-8">
            <p className="text-muted-foreground">No domains or endpoints found matching "{searchTerm}"</p>
          </div>
        )}

        {/* No Domains Message */}
        {domains.length === 0 && !searchTerm && !isLoading && !error && (
          <div className="text-center py-8">
            <p className="text-muted-foreground">No domains configured or available</p>
          </div>
        )}

        {isLoading && (
          <div className="flex items-center justify-center py-8">
            <RefreshCw className="h-6 w-6 animate-spin" />
            <span className="ml-2">Loading domain data...</span>
          </div>
        )}

        {pageBuildInfo && <Footer info={pageBuildInfo} />}
      </div>
    </div>
  )
}

export { DomainMonitorDashboard }

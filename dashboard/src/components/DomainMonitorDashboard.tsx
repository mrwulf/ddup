import { useState, useEffect, useCallback } from 'react'
import { Card, CardContent, CardHeader, CardTitle } from '@/ui/card'
import { Badge } from '@/ui/badge'
import { Button } from '@/ui/button'
import {
  RefreshCw,
  Activity,
  AlertTriangle,
  CheckCircle,
  XCircle,
  Clock,
  Search,
  Play,
  Link2,
  Cloud,
  Layers,
  Radio,
  CirclePause,
  type LucideIcon,
} from 'lucide-react'
import { cn } from '@/lib/utils'

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
}

interface DomainStatus {
  lastUpdated: string
  provider: string
  error?: string
  endpoints: DomainStatusEndpoint[]
}

type DomainsResponse = Record<string, DomainStatus>

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

// A small icon (with an optional label) that explains itself on hover; all share the same style so they read as one set
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
    {label && <span>{label}</span>}
    <span className="sr-only">{title}</span>
  </span>
)

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
      <Chip
        icon={Radio}
        title="Published in DNS"
        className="border-green-600/40 text-green-600 dark:border-green-400/40 dark:text-green-400"
      />{' '}
      Published in DNS
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
  const StatusIcon = endpoint.healthy ? CheckCircle : XCircle
  const failures = endpoint.failureCount ?? 0
  // The name is the main label when there is one, and the address is secondary
  const primary = endpoint.name || endpoint.ip
  const showTarget = !!endpoint.name

  return (
    <div className="flex flex-wrap items-center gap-x-3 gap-y-1.5 rounded-lg border p-2">
      <div className="flex min-w-0 grow basis-48 items-center gap-2">
        <StatusIcon
          className={cn(
            'h-5 w-5 shrink-0',
            endpoint.healthy ? 'text-green-600 dark:text-green-400' : 'text-red-600 dark:text-red-400'
          )}
          aria-label={endpoint.healthy ? 'Healthy' : 'Unhealthy'}
        />
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

      <div className="flex shrink-0 flex-wrap items-center gap-1.5">
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
            className={
              endpoint.active
                ? 'border-green-600/40 text-green-600 dark:border-green-400/40 dark:text-green-400'
                : undefined
            }
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

type Domain = {
  name: string
  status: DomainStatus
}

const DomainMonitorDashboard = ({ endpoint }: { endpoint: string }) => {
  const [domains, setDomains] = useState<Domain[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [lastUpdated, setLastUpdated] = useState<Date | null>(null)
  const [autoRefresh, setAutoRefresh] = useState(true)
  const [searchTerm, setSearchTerm] = useState('')
  const [isChecking, setIsChecking] = useState(false)

  const fetchDomains = useCallback(async (): Promise<void> => {
    setIsLoading(true)
    setError(null) // Clear previous errors
    try {
      const response = await fetch(endpoint + '/api/status')
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

        const response = await fetch(endpoint + '/api/status', { signal: controller.signal })
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
      const response = await fetch(endpoint + '/api/check', {
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

  const healthyDomains = filteredDomains.filter((d) => getDomainStatus(d) === 'healthy').length
  const warningDomains = filteredDomains.filter((d) => getDomainStatus(d) === 'warning').length
  const unhealthyDomains = filteredDomains.filter((d) => getDomainStatus(d) === 'unhealthy').length

  return (
    <div className="min-h-screen bg-background p-4 md:p-6">
      <div className="mx-auto max-w-7xl space-y-6">
        {/* Header */}
        <div className="flex flex-col gap-4 md:flex-row md:items-center md:justify-between">
          <div>
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
              <Button
                variant={autoRefresh ? 'default' : 'outline'}
                size="sm"
                onClick={() => setAutoRefresh(!autoRefresh)}
                className="flex items-center gap-2"
              >
                <Activity className="h-4 w-4" />
                Auto-refresh {autoRefresh ? 'ON' : 'OFF'}
              </Button>

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

              <Button
                variant="outline"
                size="sm"
                onClick={refreshClicked}
                disabled={isLoading}
                className="flex items-center gap-2"
              >
                <RefreshCw className={cn('h-4 w-4', isLoading && 'animate-spin')} />
                Refresh
              </Button>
            </div>
          </div>
        </div>

        {/* Search Bar */}
        <div className="relative max-w-md">
          <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
          <input
            type="text"
            placeholder="Search domains, endpoints or IP addresses..."
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
            className="w-full rounded-md border border-input bg-background px-10 py-2 text-sm ring-offset-background placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
          />
        </div>

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

        {/* Stats Overview */}
        {!error && domains.length > 0 && (
          <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
            <Card>
              <CardHeader className="flex flex-row items-center justify-between space-y-0">
                <CardTitle className="text-sm font-medium">Healthy Domains</CardTitle>
                <CheckCircle className="h-4 w-4 text-green-600 dark:text-green-400" />
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-bold text-green-600 dark:text-green-400">{healthyDomains}</div>
              </CardContent>
            </Card>

            <Card>
              <CardHeader className="flex flex-row items-center justify-between space-y-0">
                <CardTitle className="text-sm font-medium">Warning Domains</CardTitle>
                <AlertTriangle className="h-4 w-4 text-yellow-600 dark:text-yellow-400" />
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-bold text-yellow-600 dark:text-yellow-400">{warningDomains}</div>
              </CardContent>
            </Card>

            <Card>
              <CardHeader className="flex flex-row items-center justify-between space-y-0">
                <CardTitle className="text-sm font-medium">Unhealthy Domains</CardTitle>
                <XCircle className="h-4 w-4 text-red-600 dark:text-red-400" />
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-bold text-red-600 dark:text-red-400">{unhealthyDomains}</div>
              </CardContent>
            </Card>
          </div>
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
                    <div className="flex items-center justify-between text-xs text-muted-foreground">
                      <span>Provider: {domain.status.provider}</span>
                      <span>
                        <Clock className="inline h-3 w-3 mr-1" />
                        {new Date(domain.status.lastUpdated).toLocaleString()}
                      </span>
                    </div>
                    <div className="text-sm text-muted-foreground">
                      {domain.status.endpoints.filter((e) => e.healthy).length}/{domain.status.endpoints.length}{' '}
                      endpoints healthy
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

        {/* No Results Message */}
        {!error && domains.length > 0 && <Legend />}

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
      </div>
    </div>
  )
}

export { DomainMonitorDashboard }

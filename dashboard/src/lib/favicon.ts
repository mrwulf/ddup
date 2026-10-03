// The health shown in the browser tab: the worst status of all the domains
export type OverallStatus = 'healthy' | 'warning' | 'unhealthy' | 'unknown'

const badgeColors: Record<OverallStatus, string> = {
    healthy: '#22c55e',
    warning: '#eab308',
    unhealthy: '#ef4444',
    unknown: '#9ca3af',
}

const statusEmoji: Record<OverallStatus, string> = {
    healthy: '🟢',
    warning: '🟡',
    unhealthy: '🔴',
    unknown: '',
}

// Draws the icon, a "d" on a rounded square, with a colored badge for the status
export const faviconDataUrl = (status: OverallStatus): string | null => {
    const size = 64
    const canvas = document.createElement('canvas')
    canvas.width = size
    canvas.height = size
    const ctx = canvas.getContext('2d')
    if (!ctx) {
        return null
    }

    ctx.fillStyle = '#1e293b'
    ctx.beginPath()
    ctx.roundRect(0, 0, size, size, 14)
    ctx.fill()

    // The "d": a ring and a stem
    ctx.strokeStyle = '#fff'
    ctx.lineWidth = 7
    ctx.beginPath()
    ctx.arc(26, 34, 13, 0, Math.PI * 2)
    ctx.stroke()
    ctx.fillStyle = '#fff'
    ctx.beginPath()
    ctx.roundRect(36, 10, 7, 38, 3.5)
    ctx.fill()

    // The badge, with a ring in the color of the background so it stands out from the icon
    ctx.beginPath()
    ctx.arc(46, 46, 17, 0, Math.PI * 2)
    ctx.fillStyle = '#1e293b'
    ctx.fill()
    ctx.beginPath()
    ctx.arc(46, 46, 13, 0, Math.PI * 2)
    ctx.fillStyle = badgeColors[status]
    ctx.fill()

    return canvas.toDataURL('image/png')
}

// Title of the page, which shows the status with a colored circle, the counts of domains that need attention, and the app name
export const statusTitle = (
    status: OverallStatus,
    counts: { warning: number; unhealthy: number },
    unreachable: boolean
): string => {
    if (unreachable) {
        return '🔴 unreachable · ddup'
    }

    const parts: string[] = []
    if (counts.unhealthy > 0) {
        parts.push(`${counts.unhealthy} unhealthy`)
    }
    if (counts.warning > 0) {
        parts.push(`${counts.warning} warning`)
    }
    const emoji = statusEmoji[status]
    return [emoji && `${emoji} ${parts.join(', ')}`.trim(), 'ddup'].filter(Boolean).join(' · ')
}

// Updates the icon of the tab
export const setFavicon = (dataUrl: string): void => {
    // Replace all the icons, so the browser can't keep using the static one
    document.querySelectorAll("link[rel~='icon']").forEach((el) => el.remove())
    const link = document.createElement('link')
    link.rel = 'icon'
    link.type = 'image/png'
    link.href = dataUrl
    document.head.appendChild(link)
}

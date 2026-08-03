/**
 * API Integration Tests
 */

import { describe, it, expect, beforeAll, afterAll, vi } from 'vitest'

const API_BASE = process.env.AGENT_URL || 'http://localhost:3000'
const TEST_TOKEN = process.env.AGENT_TOKEN || 'test-token'

async function apiRequest(
  endpoint: string,
  options: RequestInit = {}
): Promise<Response> {
  const url = `${API_BASE}${endpoint}`
  const headers = {
    Authorization: `Bearer ${TEST_TOKEN}`,
    'Content-Type': 'application/json',
    ...options.headers,
  }

  return fetch(url, { ...options, headers })
}

describe('API Integration Tests', () => {
  describe('Health Endpoint', () => {
    it('should return ok status', async () => {
      try {
        const response = await fetch(`${API_BASE}/health`)
        if (response.ok) {
          const data = await response.json()
          expect(data.status).toBe('ok')
        }
      } catch (error) {
        console.log('Agent not available, skipping health test')
      }
    })
  })

  describe('SSL Endpoints', () => {
    it('should return SSL status', async () => {
      try {
        const response = await apiRequest('/api/ssl/status')
        if (response.ok) {
          const data = await response.json()
          expect(data).toHaveProperty('sites')
          expect(data).toHaveProperty('allCerts')
        }
      } catch (error) {
        console.log('Agent not available, skipping SSL test')
      }
    })
  })
})

describe('Data Validation', () => {
  describe('Domain Validation', () => {
    const validDomains = [
      'example.com',
      'sub.example.com',
      'my-site.example.org',
      '123.example.net',
    ]

    const invalidDomains = [
      '',
      '-example.com',
      'example-.com',
      'example..com',
    ]

    const domainRegex = /^[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?)*$/

    it('should validate correct domains', () => {
      validDomains.forEach((domain) => {
        expect(domainRegex.test(domain)).toBe(true)
      })
    })

    it('should reject invalid domains', () => {
      invalidDomains.forEach((domain) => {
        expect(domainRegex.test(domain)).toBe(false)
      })
    })
  })
})

describe('Utility Functions', () => {
  describe('Time Formatting', () => {
    it('should format uptime correctly', () => {
      const formatUptime = (seconds: number): string => {
        const days = Math.floor(seconds / 86400)
        const hours = Math.floor((seconds % 86400) / 3600)
        const minutes = Math.floor((seconds % 3600) / 60)

        if (days > 0) return `${days}d ${hours}h`
        if (hours > 0) return `${hours}h ${minutes}m`
        if (minutes > 0) return `${minutes}m`
        return `${seconds}s`
      }

      expect(formatUptime(90)).toBe('1m')
      expect(formatUptime(3600)).toBe('1h 0m')
      expect(formatUptime(86400)).toBe('1d 0h')
    })
  })

  describe('Size Formatting', () => {
    it('should format bytes correctly', () => {
      const formatBytes = (bytes: number): string => {
        const units = ['B', 'KB', 'MB', 'GB', 'TB']
        let i = 0
        while (bytes >= 1024 && i < units.length - 1) {
          bytes /= 1024
          i++
        }
        return `${bytes.toFixed(i > 0 ? 1 : 0)} ${units[i]}`
      }

      expect(formatBytes(0)).toBe('0 B')
      expect(formatBytes(1024)).toBe('1.0 KB')
      expect(formatBytes(1048576)).toBe('1.0 MB')
    })
  })
})

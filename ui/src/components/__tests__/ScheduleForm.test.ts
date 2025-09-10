import { vi, describe, it, expect, beforeEach } from 'vitest'
import { validateScheduleRequest, formatInterval, parseInterval } from '@/api/schedule'
import type { ExportScheduleRequest } from '@/api/schedule'

describe('ScheduleForm Utilities', () => {
  beforeEach(() => {
    vi.resetAllMocks()
  })

  describe('Form validation logic', () => {
    it('should validate correct schedule request', () => {
      const request: ExportScheduleRequest = {
        name: 'Test Schedule',
        interval_sec: 3600,
        enabled: true,
        request: {
          plugin_name: 'test-plugin',
          format: 'json',
          config: {},
          filters: {},
          options: {}
        }
      }

      const errors = validateScheduleRequest(request)
      expect(errors).toEqual([])
    })

    it('should detect validation errors', () => {
      const request: ExportScheduleRequest = {
        name: '', // Invalid: empty
        interval_sec: 30, // Invalid: too short
        enabled: true,
        request: {
          plugin_name: '', // Invalid: empty
          format: '', // Invalid: empty
          config: {},
          filters: {},
          options: {}
        }
      }

      const errors = validateScheduleRequest(request)
      
      expect(errors).toContain('Name is required')
      expect(errors).toContain('Interval must be at least 60 seconds')
      expect(errors).toContain('Plugin name is required')
      expect(errors).toContain('Format is required')
    })

    it('should detect long name', () => {
      const request: ExportScheduleRequest = {
        name: 'A'.repeat(101), // Too long
        interval_sec: 3600,
        enabled: true,
        request: {
          plugin_name: 'test',
          format: 'json',
          config: {},
          filters: {},
          options: {}
        }
      }

      const errors = validateScheduleRequest(request)
      expect(errors).toContain('Name must be less than 100 characters')
    })

    it('should detect interval too long', () => {
      const request: ExportScheduleRequest = {
        name: 'Test',
        interval_sec: 86400 * 31, // 31 days - too long
        enabled: true,
        request: {
          plugin_name: 'test',
          format: 'json',
          config: {},
          filters: {},
          options: {}
        }
      }

      const errors = validateScheduleRequest(request)
      expect(errors).toContain('Interval must be less than 30 days')
    })
  })

  describe('Interval formatting logic', () => {
    it('should format different time units correctly', () => {
      expect(formatInterval(45)).toBe('45 seconds')
      expect(formatInterval(1)).toBe('1 second')
      expect(formatInterval(90)).toBe('1 minute') // 90 seconds = 1.5 minutes -> 1 minute
      expect(formatInterval(120)).toBe('2 minutes')
      expect(formatInterval(3600)).toBe('1 hour')
      expect(formatInterval(7200)).toBe('2 hours')
      expect(formatInterval(86400)).toBe('1 day')
      expect(formatInterval(172800)).toBe('2 days')
    })

    it('should choose appropriate time unit', () => {
      // Should prefer larger units when possible
      expect(formatInterval(60)).toBe('1 minute') // Not 60 seconds
      expect(formatInterval(3600)).toBe('1 hour') // Not 60 minutes
      expect(formatInterval(86400)).toBe('1 day') // Not 24 hours
    })
  })

  describe('Interval parsing logic', () => {
    it('should parse different formats correctly', () => {
      expect(parseInterval('30 seconds')).toBe(30)
      expect(parseInterval('1 second')).toBe(1)
      expect(parseInterval('5 minutes')).toBe(300)
      expect(parseInterval('1 minute')).toBe(60)
      expect(parseInterval('2 hours')).toBe(7200)
      expect(parseInterval('1 hour')).toBe(3600)
      expect(parseInterval('3 days')).toBe(259200)
      expect(parseInterval('1 day')).toBe(86400)
    })

    it('should handle case insensitive parsing', () => {
      expect(parseInterval('5 MINUTES')).toBe(300)
      expect(parseInterval('1 Hour')).toBe(3600)
      expect(parseInterval('2 Days')).toBe(172800)
    })

    it('should throw errors for invalid formats', () => {
      expect(() => parseInterval('invalid format')).toThrow('Invalid interval format')
      expect(() => parseInterval('5 weeks')).toThrow('Invalid interval format')
      expect(() => parseInterval('')).toThrow('Invalid interval format')
      expect(() => parseInterval('abc minutes')).toThrow('Invalid interval format')
    })
  })

  describe('Interval conversion edge cases', () => {
    it('should handle boundary values', () => {
      // Minimum allowed interval
      const minRequest: ExportScheduleRequest = {
        name: 'Min Test',
        interval_sec: 60,
        enabled: true,
        request: { plugin_name: 'test', format: 'json' }
      }
      expect(validateScheduleRequest(minRequest)).toEqual([])

      // Maximum allowed interval (30 days - 1 second)
      const maxRequest: ExportScheduleRequest = {
        name: 'Max Test',
        interval_sec: 86400 * 30 - 1,
        enabled: true,
        request: { plugin_name: 'test', format: 'json' }
      }
      expect(validateScheduleRequest(maxRequest)).toEqual([])
    })

    it('should format edge case intervals', () => {
      expect(formatInterval(59)).toBe('59 seconds') // Just under 1 minute
      expect(formatInterval(3599)).toBe('59 minutes') // Just under 1 hour  
      expect(formatInterval(86399)).toBe('23 hours') // Just under 1 day
    })
  })

  describe('Form helper functions', () => {
    // Test interval conversion helpers that would be used in the form
    const convertIntervalToFormValues = (seconds: number) => {
      if (seconds % 86400 === 0) {
        return { value: seconds / 86400, unit: 'days' }
      } else if (seconds % 3600 === 0) {
        return { value: seconds / 3600, unit: 'hours' }
      } else {
        return { value: seconds / 60, unit: 'minutes' }
      }
    }

    const convertFormValuesToInterval = (value: number, unit: string) => {
      const multipliers = { minutes: 60, hours: 3600, days: 86400 }
      return value * multipliers[unit as keyof typeof multipliers]
    }

    it('should convert seconds to form values correctly', () => {
      expect(convertIntervalToFormValues(3600)).toEqual({ value: 1, unit: 'hours' })
      expect(convertIntervalToFormValues(7200)).toEqual({ value: 2, unit: 'hours' })
      expect(convertIntervalToFormValues(86400)).toEqual({ value: 1, unit: 'days' })
      expect(convertIntervalToFormValues(300)).toEqual({ value: 5, unit: 'minutes' })
    })

    it('should convert form values to seconds correctly', () => {
      expect(convertFormValuesToInterval(1, 'hours')).toBe(3600)
      expect(convertFormValuesToInterval(2, 'hours')).toBe(7200)
      expect(convertFormValuesToInterval(1, 'days')).toBe(86400)
      expect(convertFormValuesToInterval(5, 'minutes')).toBe(300)
    })

    it('should handle round-trip conversion', () => {
      const testValues = [300, 3600, 7200, 86400, 172800] // 5min, 1hr, 2hr, 1day, 2days
      
      testValues.forEach(seconds => {
        const formValues = convertIntervalToFormValues(seconds)
        const backToSeconds = convertFormValuesToInterval(formValues.value, formValues.unit)
        expect(backToSeconds).toBe(seconds)
      })
    })
  })
})
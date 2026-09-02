import { defineStore } from 'pinia'
import { getStoredPrincipal } from '@/services/auth'
import { getAccessToken } from '@/utils/storage'

export const useAppStore = defineStore('app', {
  state: () => ({
    initialized: false,
    hasStarted: false,
    authenticated: false,
    role: 'teacher' as 'parent' | 'teacher',
    summary: null as {
      academic_terms: number
      care_classes: number
      school_classes: number
      schools: number
      students: number
    } | null,
    summaryLoading: false,
  }),
  actions: {
    initialize() {
      this.initialized = true
      const token = getAccessToken()
      const principal = getStoredPrincipal()
      if (token && principal) {
        this.authenticated = true
        this.role = principal === 'parent' ? 'parent' : 'teacher'
      }
    },
    markStarted() {
      this.hasStarted = true
    },
    setRole(role: 'parent' | 'teacher') {
      this.role = role
    },
    markAuthenticated(role: 'parent' | 'teacher') {
      this.authenticated = true
      this.role = role
    },
    clearAuthenticated() {
      this.authenticated = false
    },
  },
})

# Vue.js Modernization Plan - Shelly Manager

## Executive Summary

This plan transforms Shelly Manager from 6 monolithic HTML files (9,400+ lines with 70% duplication) into a modern, maintainable Vue.js Single Page Application. The modernization will eliminate technical debt, improve user experience, and enable real-time updates through existing WebSocket infrastructure.

**Key Metrics:**
- **Code Reduction**: ~9,400 lines → ~3,500 lines (63% reduction)
- **Duplication Elimination**: 70% → <5%
- **Performance**: Expected 40-60% faster load times
- **Maintainability**: Single codebase vs. 6 separate files
- **Real-time**: Full WebSocket integration vs. current polling

## 1. Architecture Strategy

### 1.1 Technology Stack

**Core Framework:**
- **Vue 3** with Composition API (modern, performant)
- **TypeScript** for type safety and better development experience
- **Vite** as build tool (fast HMR, modern bundling)

**State Management:**
- **Pinia** (Vue 3's official state management)
- Centralized stores for devices, configuration, metrics
- Real-time state synchronization via WebSocket

**UI Framework:**
- **Quasar Framework** (Vue-based, comprehensive components)
  - Material Design 3 components
  - Built-in responsive layout system
  - Excellent form handling and validation
  - Dark/light theme support

**Additional Tools:**
- **Vue Router** for client-side navigation
- **VeeValidate** for form validation
- **Chart.js** with Vue wrapper for metrics visualization
- **Axios** for HTTP requests
- **Socket.IO** client for WebSocket communication

### 1.2 Project Structure

```
frontend/
├── src/
│   ├── components/           # Reusable UI components
│   │   ├── common/          # Common UI elements
│   │   │   ├── AppHeader.vue
│   │   │   ├── StatusIndicator.vue
│   │   │   ├── LoadingSpinner.vue
│   │   │   └── ErrorMessage.vue
│   │   ├── device/          # Device-specific components
│   │   │   ├── DeviceCard.vue
│   │   │   ├── DeviceStatus.vue
│   │   │   ├── DeviceControls.vue
│   │   │   └── DeviceConfigForm.vue
│   │   ├── forms/           # Form components
│   │   │   ├── ConfigurationForm.vue
│   │   │   ├── TemplateForm.vue
│   │   │   └── ValidationDisplay.vue
│   │   └── modals/          # Modal dialogs
│   │       ├── DeviceModal.vue
│   │       ├── ConfigModal.vue
│   │       └── ConfirmDialog.vue
│   ├── views/               # Page components
│   │   ├── Dashboard.vue    # Main dashboard
│   │   ├── DeviceConfig.vue # Device configuration
│   │   ├── Templates.vue    # Configuration templates
│   │   ├── Metrics.vue      # Analytics dashboard
│   │   ├── Setup.vue        # Initial setup wizard
│   │   └── ConfigDiff.vue   # Configuration comparison
│   ├── stores/              # Pinia stores
│   │   ├── devices.ts       # Device state management
│   │   ├── config.ts        # Configuration state
│   │   ├── templates.ts     # Template management
│   │   ├── metrics.ts       # Metrics and analytics
│   │   ├── notifications.ts # Real-time notifications
│   │   └── websocket.ts     # WebSocket connection
│   ├── services/            # API and business logic
│   │   ├── api.ts           # HTTP API client
│   │   ├── websocket.ts     # WebSocket service
│   │   ├── validation.ts    # Form validation rules
│   │   └── utils.ts         # Utility functions
│   ├── types/               # TypeScript type definitions
│   │   ├── device.ts        # Device-related types
│   │   ├── config.ts        # Configuration types
│   │   ├── api.ts           # API response types
│   │   └── index.ts         # Type exports
│   ├── styles/              # Global styles
│   │   ├── variables.scss   # Design system variables
│   │   ├── components.scss  # Component styles
│   │   └── utilities.scss   # Utility classes
│   ├── router/              # Vue Router configuration
│   │   └── index.ts
│   ├── App.vue              # Root component
│   └── main.ts              # Application entry point
├── public/                  # Static assets
├── dist/                    # Build output
├── package.json
├── vite.config.ts
├── tsconfig.json
└── README.md
```

## 2. Migration Strategy

### 2.1 Phase-by-Phase Implementation

**Phase 1: Foundation Setup (Week 1)**
- Initialize Vue 3 + Vite + TypeScript project
- Set up Quasar Framework and development environment
- Create project structure and basic routing
- Implement API service layer with TypeScript types
- Set up Pinia stores architecture

**Phase 2: Core Components (Week 2)**
- Extract reusable components from HTML files
- Implement device management components
- Create form components with validation
- Build common UI elements (header, navigation, modals)
- Establish design system and styling approach

**Phase 3: Main Views Migration (Week 3)**
- Migrate Dashboard (index.html → Dashboard.vue)
- Migrate Device Configuration (device-config.html → DeviceConfig.vue)
- Migrate Template Management (config.html → Templates.vue)
- Implement client-side routing

**Phase 4: Advanced Features (Week 4)**
- Migrate Setup Wizard (setup-wizard.html → Setup.vue)
- Migrate Metrics Dashboard (dashboard.html → Metrics.vue)
- Migrate Config Diff (config-diff.html → ConfigDiff.vue)
- Implement real-time WebSocket integration

**Phase 5: Testing & Polish (Week 5)**
- Comprehensive testing (unit, integration, e2e)
- Performance optimization
- Accessibility improvements
- Final UI/UX polish and bug fixes

### 2.2 Parallel Development Strategy

**Development Approach:**
1. **New frontend runs on port 3000** (development)
2. **Existing HTML continues on port 8080** (production)
3. **Gradual user migration** with feature flags
4. **A/B testing** for critical workflows
5. **Rollback capability** at any stage

**Integration Points:**
- Same Go backend API (no backend changes required)
- Same WebSocket endpoints for real-time updates
- Shared static assets during transition
- Configuration-driven feature flags

## 3. Technical Implementation

### 3.1 Real-time WebSocket Integration

**WebSocket Service Implementation:**
```typescript
// services/websocket.ts
import { useWebSocketStore } from '@/stores/websocket'
import { useDevicesStore } from '@/stores/devices'
import { useMetricsStore } from '@/stores/metrics'

class WebSocketService {
  private ws: WebSocket | null = null
  private reconnectAttempts = 0
  private maxReconnectAttempts = 5

  connect() {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const wsUrl = `${protocol}//${window.location.host}/metrics/ws`
    
    this.ws = new WebSocket(wsUrl)
    
    this.ws.onopen = this.onOpen.bind(this)
    this.ws.onmessage = this.onMessage.bind(this)
    this.ws.onclose = this.onClose.bind(this)
    this.ws.onerror = this.onError.bind(this)
  }

  private onMessage(event: MessageEvent) {
    const data = JSON.parse(event.data)
    
    switch (data.type) {
      case 'metrics_update':
        useMetricsStore().updateMetrics(data.data)
        break
      case 'device_status':
        useDevicesStore().updateDeviceStatus(data.device_id, data.data)
        break
      case 'notification':
        useNotificationsStore().addNotification(data.data)
        break
    }
  }
}
```

**Pinia Store with Real-time Updates:**
```typescript
// stores/devices.ts
import { defineStore } from 'pinia'
import { Device, DeviceStatus } from '@/types/device'

export const useDevicesStore = defineStore('devices', () => {
  const devices = ref<Device[]>([])
  const loading = ref(false)
  
  // Real-time status updates from WebSocket
  function updateDeviceStatus(deviceId: string, status: DeviceStatus) {
    const device = devices.value.find(d => d.id === deviceId)
    if (device) {
      device.status = status
      device.lastUpdated = new Date()
    }
  }
  
  return { devices, loading, updateDeviceStatus }
})
```

### 3.2 Form Handling Strategy

**VeeValidate Integration:**
```vue
<!-- components/forms/DeviceConfigForm.vue -->
<template>
  <q-form @submit="onSubmit" class="device-config-form">
    <!-- Device Name -->
    <q-input
      v-model="name"
      :error="!!errors.name"
      :error-message="errors.name"
      label="Device Name"
      filled
    />
    
    <!-- IP Address with validation -->
    <q-input
      v-model="ipAddress"
      :error="!!errors.ipAddress"
      :error-message="errors.ipAddress"
      label="IP Address"
      filled
      mask="###.###.###.###"
    />
    
    <!-- Dynamic configuration based on device type -->
    <component 
      :is="configComponent" 
      v-model="configuration"
      :errors="errors"
    />
    
    <!-- Submit buttons -->
    <div class="form-actions">
      <q-btn 
        type="submit" 
        color="primary" 
        :loading="loading"
        label="Save Configuration"
      />
      <q-btn 
        color="secondary" 
        outline 
        label="Test Connection"
        @click="testConnection"
      />
    </div>
  </q-form>
</template>

<script setup lang="ts">
import { useForm } from 'vee-validate'
import { deviceConfigSchema } from '@/services/validation'

const { values, errors, handleSubmit } = useForm({
  validationSchema: deviceConfigSchema
})

const onSubmit = handleSubmit(async (values) => {
  await useDevicesStore().saveDeviceConfig(props.deviceId, values)
})
</script>
```

### 3.3 Routing Strategy

**Vue Router Configuration:**
```typescript
// router/index.ts
import { createRouter, createWebHistory } from 'vue-router'

const routes = [
  {
    path: '/',
    name: 'Dashboard',
    component: () => import('@/views/Dashboard.vue'),
    meta: { title: 'Shelly Manager - Dashboard' }
  },
  {
    path: '/device/:id/config',
    name: 'DeviceConfig',
    component: () => import('@/views/DeviceConfig.vue'),
    props: true,
    meta: { title: 'Device Configuration' }
  },
  {
    path: '/templates',
    name: 'Templates',
    component: () => import('@/views/Templates.vue'),
    meta: { title: 'Configuration Templates' }
  },
  {
    path: '/metrics',
    name: 'Metrics',
    component: () => import('@/views/Metrics.vue'),
    meta: { title: 'Analytics Dashboard' }
  },
  {
    path: '/setup',
    name: 'Setup',
    component: () => import('@/views/Setup.vue'),
    meta: { title: 'Setup Wizard' }
  },
  {
    path: '/config-diff/:deviceId',
    name: 'ConfigDiff',
    component: () => import('@/views/ConfigDiff.vue'),
    props: true,
    meta: { title: 'Configuration Comparison' }
  }
]

const router = createRouter({
  history: createWebHistory(),
  routes
})

// Navigation guards for title management
router.beforeEach((to, from, next) => {
  document.title = to.meta.title as string || 'Shelly Manager'
  next()
})
```

### 3.4 Component Architecture

**Device Card Component (Eliminating Duplication):**
```vue
<!-- components/device/DeviceCard.vue -->
<template>
  <q-card class="device-card" :class="{ 'device-offline': !device.online }">
    <q-card-section>
      <div class="row items-center justify-between">
        <div>
          <div class="text-h6">{{ device.name }}</div>
          <div class="text-caption text-grey-6">{{ device.ip }}</div>
        </div>
        <status-indicator 
          :status="device.status" 
          :online="device.online"
          show-tooltip
        />
      </div>
    </q-card-section>
    
    <q-card-section v-if="showDetails">
      <!-- Power consumption for devices with meters -->
      <div v-if="device.capabilities.power_metering" class="power-metrics">
        <div class="metric">
          <span class="metric-label">Power:</span>
          <span class="metric-value">{{ device.status.power }}W</span>
        </div>
        <div class="metric">
          <span class="metric-label">Total:</span>
          <span class="metric-value">{{ device.status.total_energy }}kWh</span>
        </div>
      </div>
      
      <!-- Switch controls -->
      <div v-if="device.capabilities.relay" class="switch-controls">
        <q-toggle
          v-for="(relay, index) in device.status.relays"
          :key="index"
          v-model="relay.output"
          :label="`Relay ${index + 1}`"
          @update:model-value="toggleRelay(index, $event)"
        />
      </div>
    </q-card-section>
    
    <q-card-actions align="right">
      <q-btn 
        flat 
        color="primary" 
        label="Configure"
        :to="{ name: 'DeviceConfig', params: { id: device.id } }"
      />
      <q-btn 
        flat 
        color="secondary" 
        label="Details"
        @click="showDetails = !showDetails"
      />
    </q-card-actions>
  </q-card>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { Device } from '@/types/device'
import { useDevicesStore } from '@/stores/devices'

interface Props {
  device: Device
}

const props = defineProps<Props>()
const showDetails = ref(false)
const devicesStore = useDevicesStore()

async function toggleRelay(relayIndex: number, state: boolean) {
  await devicesStore.controlDevice(props.device.id, {
    action: 'relay',
    relay: relayIndex,
    state
  })
}
</script>
```

## 4. Development Workflow

### 4.1 Development Environment Setup

**Initial Setup Commands:**
```bash
# Create Vue project with Vite
npm create vue@latest shelly-manager-frontend

# Navigate to project
cd shelly-manager-frontend

# Install additional dependencies
npm install @quasar/cli @quasar/vite-plugin quasar
npm install pinia vue-router@4
npm install axios vee-validate yup
npm install chart.js vue-chartjs
npm install @types/node

# Install development tools
npm install -D @vue/test-utils vitest jsdom
npm install -D cypress @cypress/vue
npm install -D eslint @typescript-eslint/parser
npm install -D prettier eslint-plugin-prettier
```

**Vite Configuration:**
```typescript
// vite.config.ts
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { quasar, transformAssetUrls } from '@quasar/vite-plugin'
import { resolve } from 'path'

export default defineConfig({
  plugins: [
    vue({
      template: { transformAssetUrls }
    }),
    quasar({
      sassVariables: 'src/styles/quasar-variables.sass'
    })
  ],
  resolve: {
    alias: {
      '@': resolve(__dirname, 'src')
    }
  },
  server: {
    port: 3000,
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true
      },
      '/metrics/ws': {
        target: 'ws://localhost:8080',
        ws: true
      }
    }
  }
})
```

### 4.2 Build and Deployment Process

**Development Workflow:**
```bash
# Development server with hot reload
npm run dev

# Type checking
npm run type-check

# Linting
npm run lint

# Testing
npm run test:unit
npm run test:e2e

# Production build
npm run build

# Preview production build
npm run preview
```

**Deployment Integration:**
```bash
# Build script for production
#!/bin/bash
cd frontend
npm ci --only=production
npm run build

# Copy built assets to Go static directory
cp -r dist/* ../ui/dist/

# Update Go routes to serve SPA
echo "Updating Go server for SPA routing..."
```

### 4.3 Testing Strategy

**Unit Testing with Vitest:**
```typescript
// tests/components/DeviceCard.test.ts
import { mount } from '@vue/test-utils'
import { createTestingPinia } from '@pinia/testing'
import DeviceCard from '@/components/device/DeviceCard.vue'

describe('DeviceCard', () => {
  const mockDevice = {
    id: '1',
    name: 'Test Device',
    ip: '192.168.1.100',
    online: true,
    status: { power: 15.5 },
    capabilities: { power_metering: true }
  }

  it('displays device information correctly', () => {
    const wrapper = mount(DeviceCard, {
      props: { device: mockDevice },
      global: {
        plugins: [createTestingPinia()]
      }
    })

    expect(wrapper.text()).toContain('Test Device')
    expect(wrapper.text()).toContain('192.168.1.100')
    expect(wrapper.text()).toContain('15.5W')
  })
})
```

**E2E Testing with Cypress:**
```typescript
// cypress/e2e/device-management.cy.ts
describe('Device Management', () => {
  beforeEach(() => {
    cy.visit('/')
  })

  it('should display device list', () => {
    cy.get('[data-cy=device-card]').should('have.length.greaterThan', 0)
  })

  it('should navigate to device configuration', () => {
    cy.get('[data-cy=device-card]').first().find('[data-cy=configure-btn]').click()
    cy.url().should('include', '/device/')
    cy.url().should('include', '/config')
  })

  it('should toggle device relay', () => {
    cy.get('[data-cy=relay-toggle]').first().click()
    cy.get('[data-cy=status-message]').should('contain', 'Relay updated')
  })
})
```

## 5. Timeline and Milestones

### 5.1 Detailed Implementation Timeline

**Week 1: Foundation Setup**
- Day 1-2: Project setup, tooling configuration
- Day 3-4: API service layer, TypeScript types
- Day 5-7: Pinia stores architecture, routing setup

**Week 2: Core Components**
- Day 1-2: Common UI components (header, navigation, modals)
- Day 3-4: Device management components
- Day 5-7: Form components with validation

**Week 3: Main Views Migration**
- Day 1-2: Dashboard view (main device management)
- Day 3-4: Device Configuration view
- Day 5-7: Template Management view

**Week 4: Advanced Features**
- Day 1-2: Setup Wizard migration
- Day 3-4: Metrics Dashboard with charts
- Day 5-7: Config Diff view, real-time WebSocket integration

**Week 5: Testing & Polish**
- Day 1-2: Unit tests, integration tests
- Day 3-4: E2E tests, performance optimization
- Day 5-7: Final polish, accessibility improvements, deployment

### 5.2 Success Metrics

**Performance Metrics:**
- Initial load time: < 2 seconds (target: 1.5s)
- Time to interactive: < 3 seconds (target: 2s)
- Bundle size: < 500KB gzipped (target: 300KB)
- Lighthouse score: > 90 (target: 95+)

**User Experience Metrics:**
- Navigation between sections: < 200ms
- Form submission response: < 1 second
- Real-time update latency: < 100ms
- Zero context loss during navigation

**Code Quality Metrics:**
- Test coverage: > 80% (target: 90%)
- TypeScript coverage: 100%
- ESLint violations: 0
- Accessibility score: WCAG 2.1 AA compliance

### 5.3 Risk Mitigation Strategies

**Technical Risks:**
- **WebSocket Integration Issues**: Implement fallback to HTTP polling
- **Performance Degradation**: Implement code splitting, lazy loading
- **Browser Compatibility**: Target modern browsers, provide polyfills
- **State Management Complexity**: Start simple, incrementally add complexity

**Migration Risks:**
- **Feature Parity**: Detailed feature comparison checklist
- **Data Loss**: Comprehensive testing with production data
- **User Training**: In-app guided tours, documentation
- **Rollback Plan**: Feature flags, deployment strategy

**Timeline Risks:**
- **Scope Creep**: Strict feature freeze after Week 3
- **Technical Debt**: Allocate 20% time for refactoring
- **Testing Delays**: Parallel development and testing
- **Integration Issues**: Daily integration testing

## 6. Implementation Details

### 6.1 Component Extraction Strategy

**From HTML to Vue Components:**

1. **Identify Repeated Patterns**: Extract header, navigation, forms, status displays
2. **Create Base Components**: Build foundation components first
3. **Compose Views**: Combine components into page views
4. **Progressive Enhancement**: Add features incrementally

**Example Extraction - Device Status:**
```html
<!-- Before: Repeated in multiple HTML files -->
<div class="device-status">
    <span class="status-indicator online"></span>
    <div class="device-info">
        <h3>Device Name</h3>
        <p>192.168.1.100</p>
    </div>
</div>
```

```vue
<!-- After: Reusable Vue component -->
<template>
  <div class="device-status">
    <status-indicator :online="device.online" />
    <div class="device-info">
      <h3>{{ device.name }}</h3>
      <p>{{ device.ip }}</p>
    </div>
  </div>
</template>
```

### 6.2 State Management Architecture

**Store Organization:**
```typescript
// stores/devices.ts - Device state management
export const useDevicesStore = defineStore('devices', () => {
  // State
  const devices = ref<Device[]>([])
  const selectedDevice = ref<Device | null>(null)
  const loading = ref(false)
  
  // Getters
  const onlineDevices = computed(() => 
    devices.value.filter(d => d.online)
  )
  
  // Actions
  async function fetchDevices() {
    loading.value = true
    try {
      const response = await api.get('/devices')
      devices.value = response.data
    } finally {
      loading.value = false
    }
  }
  
  return { devices, selectedDevice, loading, onlineDevices, fetchDevices }
})
```

### 6.3 API Integration Layer

**Centralized API Service:**
```typescript
// services/api.ts
import axios from 'axios'

const api = axios.create({
  baseURL: '/api/v1',
  timeout: 10000
})

// Response interceptor for error handling
api.interceptors.response.use(
  (response) => response,
  (error) => {
    // Global error handling
    console.error('API Error:', error)
    throw error
  }
)

export const deviceAPI = {
  getDevices: () => api.get('/devices'),
  getDevice: (id: string) => api.get(`/devices/${id}`),
  updateDevice: (id: string, data: any) => api.put(`/devices/${id}`, data),
  controlDevice: (id: string, action: any) => api.post(`/devices/${id}/control`, action)
}
```

## 7. Post-Migration Benefits

### 7.1 Immediate Benefits
- **70% reduction in code duplication**
- **Single source of truth for UI components**
- **Real-time updates without page refreshes**
- **Consistent user experience across all sections**
- **Modern development workflow with hot reload**

### 7.2 Long-term Benefits
- **Easier feature additions** (component-based architecture)
- **Better testing capabilities** (isolated component testing)
- **Improved performance** (code splitting, lazy loading)
- **Enhanced accessibility** (modern frameworks, ARIA support)
- **Mobile responsiveness** (Quasar's responsive components)

### 7.3 Developer Experience Improvements
- **TypeScript** for better code quality and IDE support
- **Component-based development** for better code organization
- **Hot module replacement** for faster development cycles
- **Comprehensive testing tools** for reliable code
- **Modern debugging tools** (Vue DevTools)

## 8. Conclusion

This Vue.js modernization plan provides a comprehensive roadmap for transforming Shelly Manager from a legacy HTML-based application to a modern, maintainable Single Page Application. The phased approach ensures minimal risk while delivering substantial improvements in code quality, user experience, and developer productivity.

**Key Success Factors:**
1. **Incremental Migration**: Parallel development with gradual transition
2. **Component-First Approach**: Eliminate duplication through reusable components
3. **Real-time Integration**: Leverage existing WebSocket infrastructure
4. **Comprehensive Testing**: Ensure reliability through automated testing
5. **Performance Focus**: Modern build tools and optimization techniques

The plan balances ambitious modernization goals with practical implementation constraints, ensuring successful delivery within the 5-week timeline while maintaining production stability.

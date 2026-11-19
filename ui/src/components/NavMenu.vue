<template>
  <div
    ref="root"
    class="nav-dropdown"
    :class="{ open: isOpen }"
    @focusout="onFocusOut"
    @pointerdown="pressInside = true"
  >
    <button
      ref="trigger"
      type="button"
      class="nav-link dropdown-trigger"
      :class="{ active }"
      :aria-expanded="isOpen"
      @click="isOpen = !isOpen"
    >
      {{ label }}
    </button>
    <div class="dropdown-menu" role="menu" :aria-hidden="!isOpen">
      <template v-for="(section, index) in sections" :key="index">
        <div v-if="index > 0" class="dropdown-divider"></div>
        <div class="dropdown-section">
          <div v-if="section.title" class="dropdown-section-title">{{ section.title }}</div>
          <router-link
            v-for="item in section.items"
            :key="item.to"
            class="dropdown-item"
            :to="item.to"
            role="menuitem"
            @click="close"
          >
            <span v-if="item.icon" class="dropdown-icon" aria-hidden="true">{{ item.icon }}</span>
            {{ item.label }}
          </router-link>
        </div>
      </template>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'

export interface NavMenuItem {
  label: string
  to: string
  icon?: string
}

export interface NavMenuSection {
  title?: string
  items: NavMenuItem[]
}

defineProps<{
  label: string
  active: boolean
  sections: NavMenuSection[]
}>()

const root = ref<HTMLElement | null>(null)
const trigger = ref<HTMLButtonElement | null>(null)
const isOpen = ref(false)

// A pointer press inside the menu blurs the trigger with no related target in some browsers, which
// is indistinguishable from focus going nowhere; the flag tells the two apart. It is set by a press
// inside the menu and cleared by the release or cancel of any press, wherever it ends.
const pressInside = ref(false)

function close() {
  isOpen.value = false
}

function onFocusOut(event: FocusEvent) {
  const next = event.relatedTarget as Node | null
  if (next && root.value?.contains(next)) return
  if (pressInside.value) return
  close()
}

function onDocumentPointerDown(event: Event) {
  if (!root.value?.contains(event.target as Node)) close()
}

function onDocumentPointerEnd() {
  pressInside.value = false
}

// Escape closes an open menu wherever the focus is; focus goes back to the button only when it was
// inside the menu, since otherwise it would be left on a hidden item.
function onDocumentKeydown(event: KeyboardEvent) {
  if (event.key !== 'Escape' || !isOpen.value) return
  const focusInside = root.value?.contains(document.activeElement)
  close()
  if (focusInside) trigger.value?.focus()
}

onMounted(() => {
  document.addEventListener('pointerdown', onDocumentPointerDown)
  document.addEventListener('pointerup', onDocumentPointerEnd)
  document.addEventListener('pointercancel', onDocumentPointerEnd)
  document.addEventListener('keydown', onDocumentKeydown)
})
onBeforeUnmount(() => {
  document.removeEventListener('pointerdown', onDocumentPointerDown)
  document.removeEventListener('pointerup', onDocumentPointerEnd)
  document.removeEventListener('pointercancel', onDocumentPointerEnd)
  document.removeEventListener('keydown', onDocumentKeydown)
})
</script>

<style scoped>
.nav-dropdown {
  position: relative;
  display: inline-block;
}

.dropdown-trigger {
  /* Reset button styling for accessibility */
  background: none;
  border: none;
  cursor: pointer;
  font: inherit;
  padding: 8px 16px;
  display: flex;
  align-items: center;
  user-select: none;
}

.dropdown-trigger:after {
  content: ' ▼';
  font-size: 10px;
  margin-left: 4px;
  transition: transform 0.2s ease;
}

.nav-dropdown.open .dropdown-trigger:after {
  transform: rotate(180deg);
}

.dropdown-menu {
  display: none;
  position: absolute;
  top: 100%;
  left: 0;
  min-width: 220px;
  background: #fff;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
  box-shadow: 0 10px 25px rgba(0, 0, 0, 0.15);
  z-index: 1000;
  margin-top: 4px;
  overflow: hidden;
}

.nav-dropdown.open .dropdown-menu {
  display: block;
  animation: fadeIn 0.15s ease-out;
}

@keyframes fadeIn {
  from { opacity: 0; transform: translateY(-8px); }
  to { opacity: 1; transform: translateY(0); }
}

.dropdown-section {
  padding: 8px 0;
}

.dropdown-section-title {
  font-size: 11px;
  font-weight: 600;
  color: #6b7280;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  padding: 4px 16px;
  margin-bottom: 4px;
}

.dropdown-divider {
  height: 1px;
  background: #f3f4f6;
  margin: 4px 0;
}

.dropdown-item {
  display: flex;
  align-items: center;
  padding: 10px 16px;
  color: #374151;
  text-decoration: none;
  font-size: 14px;
  transition: all 0.15s ease;
}

.dropdown-item:hover {
  background: #f8fafc;
  color: #1f2937;
}

.dropdown-item.router-link-active {
  background: #eff6ff;
  color: #2563eb;
  font-weight: 500;
}

.dropdown-icon {
  margin-right: 8px;
  font-size: 16px;
  width: 20px;
  text-align: center;
}

@media (max-width: 768px) {
  .dropdown-menu {
    min-width: 200px;
    right: 0;
    left: auto;
  }
}
</style>

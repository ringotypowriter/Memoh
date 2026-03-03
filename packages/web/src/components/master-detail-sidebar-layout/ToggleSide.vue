<template>
  <FontAwesomeIcon
    :icon="['fas', 'bars']"
    :class="`m-auto ${isMobile?'':'hidden!'}`"
    @click="toggleSidebar"
  />
</template>
<script setup lang="ts">
import { useSidebar } from '@memoh/ui'

const { toggleSidebar, isMobile } = useSidebar()

</script>

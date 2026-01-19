<script setup lang="ts">
// import type { Payment } from '@/components/columns'
import { ref, h } from 'vue'
import CreateModel from '@/components/CreateModel/index.vue'

import DataTable from '@/components/DataTable/index.vue'

const modelData=ref([])

async function getData() {

  return [
    {
      id: '728ed52f',
      amount: 100,
      status: 'pending',
      email: 'm@example.com',
    },

  ]
}

const columns = [
  {
    accessorKey: 'amount',
    header: () => h('div', { class: 'text-right' }, 'Amount'),
    
  }
]

</script>

<template>
  <div class="w-full py-10 mx-auto">
    <div class="flex mb-4">
      <CreateModel />
    </div>
    <DataTable
      :columns="columns"
      :data="modelData"
    />
  </div>
</template>
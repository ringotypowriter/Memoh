<template>
  <section class="ml-auto">
    <Dialog v-model:open="open">
      <DialogTrigger as-child>
        <Button variant="default">
          Open Dialog
        </Button>
      </DialogTrigger>
      <DialogContent class="sm:max-w-106.25">
        <form @submit="addModel">
          <DialogHeader>
            <DialogTitle>添加Model</DialogTitle>
            <DialogDescription class="mb-4">
              使用不用厂商的大模型
            </DialogDescription>
          </DialogHeader>
          <div class="grid ">
            <FormField
              v-slot="{ componentField }"
              name="baseUrl"
            >
              <FormItem>
                <FormLabel class="mb-2">
                  Base Url
                </FormLabel>
                <FormControl>
                  <Input                   
                    type="text"
                    placeholder="请输入Base Url"
                    v-bind="componentField"
                    autocomplete="baseurl"
                  />
                </FormControl>
                <blockquote class="h-5">
                  <FormMessage />
                </blockquote>
              </FormItem>
            </FormField>
            <FormField
              v-slot="{ componentField }"
              name="apiKey"
            >
              <FormItem>
                <FormLabel class="mb-2">
                  Api Key
                </FormLabel>
                <FormControl>
                  <Input                   
                    placeholder="请输入Api Key"
                    autocomplete="apiKey"
                    v-bind="componentField"
                  />
                </FormControl>
                <blockquote class="h-5">
                  <FormMessage />
                </blockquote>
              </FormItem>
            </FormField>
            <FormField
              v-slot="{ componentField }"
              name="clientType"
            >
              <FormItem>
                <FormLabel class="mb-2">
                  Client Type
                </FormLabel>
                <FormControl>
                  <Input                    
                    placeholder="请输入Api Key"
                    autocomplete="clientType"
                    v-bind="componentField"
                  />
                </FormControl>
                <blockquote class="h-5">
                  <FormMessage />
                </blockquote>
              </FormItem>
            </FormField>
            <FormField
              v-slot="{ componentField }"
              name="name"
            >
              <FormItem>
                <FormLabel class="mb-2">
                  Name
                </FormLabel>
                <FormControl>
                  <Input
                    placeholder="请输入Api Key"
                    autocomplete="name"
                    v-bind="componentField"
                  />
                </FormControl>
                <blockquote class="h-5">
                  <FormMessage />
                </blockquote>
              </FormItem>
            </FormField>
            <FormField
              v-slot="{ componentField }"
              name="role"
            >
              <FormItem>
                <FormLabel class="mb-2">
                  Role
                </FormLabel>
                <FormControl>
                  <Select v-bind="componentField">
                    <SelectTrigger
                      class="w-full"
                    >
                      <SelectValue
                        placeholder="Select a fruit"
                      />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectGroup>
                        <SelectItem
                          value="chat"
                          select
                        >
                          Chat
                        </SelectItem>
                      </SelectGroup>
                    </SelectContent>
                  </Select>
                </FormControl>
                <blockquote class="h-5">
                  <FormMessage />
                </blockquote>
              </FormItem>
            </FormField>
          </div>
          <DialogFooter class="mt-4">
            <DialogClose as-child>
              <Button variant="outline">
                Cancel
              </Button>
            </DialogClose>
            <Button type="submit">
              添加Model
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  </section>
</template>

<script setup lang="ts">
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
  Input,
  Button,
  FormField,
  FormControl,
  Select,
  SelectContent,
  SelectGroup,
  SelectItem, 
  SelectTrigger,
  SelectValue,
  FormItem,
  FormLabel,
  FormMessage
} from '@memoh/ui'
import { useForm } from 'vee-validate'
import { ref } from 'vue'
import { toTypedSchema } from '@vee-validate/zod'
import z from 'zod'
import request from '@/utils/request'

const formSchema = toTypedSchema(z.object({
  baseUrl: z.string().min(1),
  apiKey: z.string().min(1),
  clientType: z.string().min(1),
  name: z.string().min(1),
  role: z.string().min(1),
}))

const form = useForm({
  validationSchema:formSchema
})
const addModel=form.handleSubmit(async (modelInfo) => {
  try {
    await request({
      url: '/model',
      data: {
        ...modelInfo
      }
    })  
    open.value = false
  } catch (err) {
    return err
  }
  
 
})

const open=ref(false)
</script>
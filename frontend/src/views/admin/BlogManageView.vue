<template>
  <AppLayout>
    <TablePageLayout>
      <template #filters>
        <div class="flex flex-wrap items-center gap-3">
          <div class="flex-1 sm:max-w-64">
            <input
              v-model="searchQuery"
              type="text"
              :placeholder="t('admin.blog.searchPlaceholder')"
              class="input"
              @input="handleSearch"
            />
          </div>
          <Select
            v-model="filters.status"
            :options="statusFilterOptions"
            class="w-40"
            @change="handleStatusChange"
          />

          <div class="flex flex-1 flex-wrap items-center justify-end gap-2">
            <button
              @click="loadBlogs"
              :disabled="loading"
              class="btn btn-secondary"
              :title="t('common.refresh')"
            >
              <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
            </button>
            <button @click="openCreate" class="btn btn-primary">
              <Icon name="plus" size="md" class="mr-1" />
              {{ t('admin.blog.createBlog') }}
            </button>
          </div>
        </div>
      </template>

      <template #table>
        <DataTable
          :columns="columns"
          :data="blogs"
          :loading="loading"
        >
          <template #cell-title="{ row }">
            <div class="min-w-0">
              <div class="truncate font-medium text-gray-900 dark:text-white">{{ row.title }}</div>
              <div class="mt-1 text-xs text-gray-500 dark:text-dark-400">
                #{{ row.id }} · {{ formatDateTime(row.created_at) }}
              </div>
            </div>
          </template>
          <template #cell-status="{ value }">
            <span
              :class="[
                'badge',
                value === 'published' ? 'badge-success' : 'badge-gray'
              ]"
            >
              {{ value === 'published' ? t('admin.blog.statusPublished') : t('admin.blog.statusDraft') }}
            </span>
          </template>
          <template #cell-tags="{ value }">
            <span v-if="value" class="text-sm text-gray-600 dark:text-dark-300">{{ value }}</span>
            <span v-else class="text-xs text-gray-400">-</span>
          </template>
          <template #cell-published_at="{ value }">
            <span class="text-sm text-gray-600 dark:text-dark-300">
              {{ value ? formatDateTime(value) : '-' }}
            </span>
          </template>
          <template #cell-actions="{ row }">
            <div class="flex items-center justify-end gap-2">
              <button class="btn btn-secondary btn-sm" @click="openEdit(row)">
                {{ t('common.edit') }}
              </button>
              <button class="btn btn-danger btn-sm" @click="confirmDelete(row)">
                {{ t('common.delete') }}
              </button>
            </div>
          </template>
        </DataTable>
      </template>

      <template #pagination>
        <Pagination
          v-if="pagination.total > 0"
          :page="pagination.page"
          :total="pagination.total"
          :page-size="pagination.page_size"
          @update:page="handlePageChange"
          @update:pageSize="handlePageSizeChange"
        />
      </template>
    </TablePageLayout>

    <!-- 编辑/创建对话框 -->
    <div v-if="editing" class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4" @click.self="closeDialog">
      <div class="flex max-h-[90vh] w-full max-w-4xl flex-col overflow-hidden rounded-2xl bg-white shadow-xl dark:bg-dark-900">
        <div class="flex items-center justify-between border-b border-gray-200 px-6 py-4 dark:border-dark-700">
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
            {{ editing.id ? t('admin.blog.editBlog') : t('admin.blog.createBlog') }}
          </h2>
          <button class="text-gray-400 hover:text-gray-600 dark:hover:text-white" @click="closeDialog">×</button>
        </div>
        <div class="flex-1 space-y-4 overflow-y-auto px-6 py-5">
          <div>
            <label class="form-label">{{ t('admin.blog.fields.title') }}</label>
            <input v-model="editing.title" type="text" class="input" maxlength="500" />
          </div>
          <div>
            <label class="form-label">{{ t('admin.blog.fields.summary') }}</label>
            <textarea v-model="editing.summary" rows="4" class="textarea" maxlength="1000" />
          </div>
          <div>
            <label class="form-label">{{ t('admin.blog.fields.coverImage') }}</label>
            <div class="flex items-start gap-3">
              <div
                v-if="editing.cover_image"
                class="h-20 w-32 flex-shrink-0 overflow-hidden rounded-lg border border-gray-200 bg-gray-50 dark:border-dark-700 dark:bg-dark-800"
              >
                <img :src="editing.cover_image" alt="" class="h-full w-full object-cover" />
              </div>
              <div class="min-w-0 flex-1 space-y-2">
                <div class="flex gap-2">
                  <input
                    ref="coverFileInput"
                    type="file"
                    accept="image/png,image/jpeg,image/gif,image/webp"
                    class="hidden"
                    @change="handleCoverFile"
                  />
                  <button
                    type="button"
                    class="btn btn-secondary btn-sm"
                    :disabled="coverUploading"
                    @click="coverFileInput?.click()"
                  >
                    {{ coverUploading ? t('admin.blog.uploading') : t('admin.blog.uploadCover') }}
                  </button>
                  <button
                    v-if="editing.cover_image"
                    type="button"
                    class="btn btn-secondary btn-sm"
                    @click="editing.cover_image = ''"
                  >
                    {{ t('admin.blog.removeCover') }}
                  </button>
                </div>
                <input
                  v-model="editing.cover_image"
                  type="text"
                  class="input"
                  maxlength="1000"
                  :placeholder="t('admin.blog.coverUrlPlaceholder')"
                />
              </div>
            </div>
            <p v-if="coverUploadError" class="mt-1 text-xs text-red-500">{{ coverUploadError }}</p>
          </div>
          <div>
            <label class="form-label">{{ t('admin.blog.fields.tags') }}</label>
            <input v-model="editing.tags" type="text" class="input" :placeholder="t('admin.blog.tagsPlaceholder')" />
          </div>
          <div>
            <label class="form-label">{{ t('admin.blog.fields.content') }}</label>
            <div class="mb-2 flex flex-wrap items-center gap-2">
              <input
                ref="contentImageInput"
                type="file"
                accept="image/png,image/jpeg,image/gif,image/webp,image/bmp"
                class="hidden"
                @change="onContentImagePick"
              />
              <button
                type="button"
                class="btn btn-secondary btn-sm"
                :disabled="contentUploading"
                @click="contentImageInput?.click()"
              >
                {{ contentUploading ? t('admin.blog.uploading') : t('admin.blog.insertImage') }}
              </button>
              <span class="text-xs text-gray-400">{{ t('admin.blog.insertImageHint') }}</span>
            </div>
            <div class="overflow-hidden rounded-lg border border-gray-300 dark:border-dark-600">
              <Toolbar
                :key="`toolbar-${editorKey}`"
                :editor="editorRef"
                :default-config="toolbarConfig"
                mode="default"
                class="border-b border-gray-200 dark:border-dark-700"
              />
              <Editor
                :key="`editor-${editorKey}`"
                v-model="editing.content"
                :default-config="editorConfig"
                mode="default"
                class="min-h-[320px]"
                @on-created="handleEditorCreated"
              />
            </div>
          </div>
          <div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
            <div>
              <label class="form-label">{{ t('admin.blog.fields.status') }}</label>
              <Select
                v-model="editing.status"
                :options="statusOptions"
                class="w-full"
              />
            </div>
            <div>
              <label class="form-label">{{ t('admin.blog.fields.publishedAt') }}</label>
              <input
                v-model="editing.publishedAtInput"
                type="datetime-local"
                class="input"
              />
              <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">
                {{ t('admin.blog.publishedAtHint') }}
              </p>
            </div>
          </div>
        </div>
        <div class="flex justify-end gap-2 border-t border-gray-200 px-6 py-4 dark:border-dark-700">
          <button class="btn btn-secondary" @click="closeDialog">{{ t('common.cancel') }}</button>
          <button class="btn btn-primary" :disabled="saving" @click="saveBlog">
            {{ saving ? t('common.saving') : t('common.save') }}
          </button>
        </div>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, shallowRef, computed, reactive } from 'vue'
import { useI18n } from 'vue-i18n'
import '@wangeditor/editor/dist/css/style.css'
import { Editor, Toolbar } from '@wangeditor/editor-for-vue'
import { i18nChangeLanguage } from '@wangeditor/editor'
import type { IDomEditor, IEditorConfig, IToolbarConfig } from '@wangeditor/editor'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import Select from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'
import { adminBlogAPI } from '@/api'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'
import { formatDateTime } from '@/utils/format'
import type { Blog, BlogStatus } from '@/types'

const { t, locale } = useI18n()

interface EditingBlog {
  id?: number
  title: string
  content: string
  summary: string
  cover_image: string
  status: BlogStatus
  tags: string
  publishedAtInput: string // datetime-local string, '' means cleared
}

const blogs = ref<Blog[]>([])
const loading = ref(false)
const saving = ref(false)
const searchQuery = ref('')
const filters = ref<{ status: '' | BlogStatus }>({ status: '' })
const editing = ref<EditingBlog | null>(null)

const pagination = reactive({
  page: 1,
  page_size: getPersistedPageSize(),
  total: 0
})

// ============ 富文本编辑器 ============
// 编辑器实例必须用 shallowRef，否则 Vue 的深层响应式会破坏 wangEditor 内部结构
const editorRef = shallowRef<IDomEditor>()
const editorKey = ref(0)
const coverFileInput = ref<HTMLInputElement | null>(null)
const coverUploading = ref(false)
const coverUploadError = ref('')
const contentImageInput = ref<HTMLInputElement | null>(null)
const contentUploading = ref(false)

const toolbarConfig: Partial<IToolbarConfig> = {
  toolbarKeys: [
    'headerSelect',
    'blockquote',
    '|',
    'bold',
    'italic',
    'underline',
    'strikeThrough',
    'sub',
    'sup',
    '|',
    'fontFamily',
    'fontSize',
    'lineHeight',
    '|',
    'color',
    'bgColor',
    '|',
    'bulletedList',
    'numberedList',
    'todo',
    '|',
    'justifyLeft',
    'justifyCenter',
    'justifyRight',
    'justifyJustify',
    'indent',
    'delIndent',
    '|',
    'emotion',
    'link',
    'uploadImage',
    'insertTable',
    'codeBlock',
    'divider',
    '|',
    'undo',
    'redo',
    'clearStyle',
    'fullScreen'
  ]
}

const editorConfig: Partial<IEditorConfig> = {
  placeholder: 'Write something...',
  MENU_CONF: {
    fontFamily: {
      fontFamilyList: [
        'Arial',
        'Tahoma',
        'Verdana',
        'Times New Roman',
        'Georgia',
        'Courier New',
        'Helvetica',
        'PingFang SC',
        'Microsoft YaHei',
        'sans-serif'
      ]
    },
    fontSize: {
      fontSizeList: [
        '12px', '13px', '14px', '15px', '16px', '18px', '20px', '24px', '28px', '32px', '40px'
      ]
    },
    lineHeight: {
      lineHeightList: ['1', '1.15', '1.5', '2', '2.5', '3']
    },
    uploadImage: {
      allowedFileTypes: ['image/jpeg', 'image/png', 'image/gif', 'image/webp', 'image/bmp'],
      maxFileSize: 10 * 1024 * 1024,
      // 禁用 base64 内联（默认已是 0）：图片一律走服务器上传，避免正文被 base64 撑爆
      base64LimitSize: 0,
      // 自定义上传：走我们自己后端的 /api/v1/admin/uploads/image
      async customUpload(file: File, insertFn: (url: string, alt: string, href: string) => void) {
        try {
          const url = await adminBlogAPI.uploadImage(file)
          if (!url) throw new Error('empty upload url')
          insertBlogImage(url, file.name, insertFn)
        } catch (err: any) {
          console.error('Failed to upload blog image', err)
          alert(err?.message || t('admin.blog.imageUploadFailed'))
        }
      }
    }
  }
}

function handleEditorCreated(editor: IDomEditor) {
  editorRef.value = editor
}

function escapeHtmlAttr(value: string): string {
  return value.replace(/[&<>"']/g, (ch) => {
    switch (ch) {
      case '&': return '&amp;'
      case '<': return '&lt;'
      case '>': return '&gt;'
      case '"': return '&quot;'
      default: return '&#39;'
    }
  })
}

/**
 * 把上传成功的图片插入正文。
 *
 * wangEditor 的插入在「没有有效选区」时会静默失败（既不插入也不报错）：
 * 典型场景是用户还没点过编辑区就直接点了工具栏的上传按钮，此时 editor.selection 为 null。
 * 所以这里插入前先恢复/建立选区，插入后再校验一次，失败则用 HTML 直接插入兜底。
 */
function insertBlogImage(
  url: string,
  alt: string,
  insertFn: (url: string, alt: string, href: string) => void
) {
  const editor = editorRef.value
  const alive = !!editor && !editor.isDestroyed

  // 用图片数量比对，避免 wangEditor 内部改写 src 导致误判
  const countImages = (html: string) => (html.match(/<img\b/gi) || []).length
  const before = alive ? countImages(editor!.getHtml()) : -1

  if (alive) {
    try {
      if (!editor!.selection) {
        editor!.restoreSelection()
      }
      // 仍无选区：把光标放到正文末尾
      if (!editor!.selection) {
        editor!.focus(true)
      }
    } catch (err) {
      console.warn('Failed to prepare editor selection', err)
    }
  }

  insertFn(url, alt, '')

  // 兜底：确认图片真的进正文了，否则改用 HTML 插入
  if (alive) {
    try {
      if (countImages(editor!.getHtml()) === before) {
        editor!.focus(true)
        editor!.dangerouslyInsertHtml(
          `<img src="${escapeHtmlAttr(url)}" alt="${escapeHtmlAttr(alt)}" style="max-width:100%;"/>`
        )
      }
    } catch (err) {
      console.warn('Fallback image insert failed', err)
    }
  }
}

function destroyEditor() {
  const editor = editorRef.value
  if (editor && !editor.isDestroyed) {
    editor.destroy()
  }
  editorRef.value = undefined
}

// 正文插入图片：独立于 wangEditor 工具栏按钮，永远可用（走同一套上传+插入逻辑）
async function onContentImagePick(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return
  const editor = editorRef.value
  contentUploading.value = true
  try {
    const url = await adminBlogAPI.uploadImage(file)
    if (!url) throw new Error('empty upload url')
    if (editor && !editor.isDestroyed) {
      if (!editor.selection) editor.restoreSelection()
      if (!editor.selection) editor.focus(true)
      editor.dangerouslyInsertHtml(
        `<img src="${escapeHtmlAttr(url)}" alt="${escapeHtmlAttr(file.name)}" style="max-width:100%;"/>`
      )
    } else {
      alert(t('admin.blog.imageUploadFailed'))
    }
  } catch (err: any) {
    console.error('Failed to insert content image', err)
    alert(err?.message || t('admin.blog.imageUploadFailed'))
  } finally {
    contentUploading.value = false
    input.value = ''
  }
}

// 封面图上传
async function handleCoverFile(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file || !editing.value) return
  coverUploadError.value = ''
  coverUploading.value = true
  try {
    const url = await adminBlogAPI.uploadImage(file)
    if (!url) throw new Error('empty upload url')
    editing.value.cover_image = url
  } catch (err: any) {
    console.error('Failed to upload cover image', err)
    coverUploadError.value = err?.message || t('admin.blog.imageUploadFailed')
  } finally {
    coverUploading.value = false
    // 允许重复选择同一个文件
    input.value = ''
  }
}

const statusOptions = computed(() => [
  { value: 'draft', label: t('admin.blog.statusDraft') },
  { value: 'published', label: t('admin.blog.statusPublished') }
])

const statusFilterOptions = computed(() => [
  { value: '', label: t('admin.blog.allStatuses') },
  { value: 'draft', label: t('admin.blog.statusDraft') },
  { value: 'published', label: t('admin.blog.statusPublished') }
])

const columns = [
  { key: 'title', label: t('admin.blog.fields.title'), sortable: false },
  { key: 'status', label: t('admin.blog.fields.status'), sortable: false },
  { key: 'tags', label: t('admin.blog.fields.tags'), sortable: false },
  { key: 'published_at', label: t('admin.blog.fields.publishedAt'), sortable: false },
  { key: 'actions', label: t('admin.blog.fields.actions'), sortable: false, align: 'right' as const }
]

let searchTimer: ReturnType<typeof setTimeout> | null = null

async function loadBlogs() {
  loading.value = true
  try {
    const result = await adminBlogAPI.list({
      page: pagination.page,
      page_size: pagination.page_size,
      status: filters.value.status || undefined,
      search: searchQuery.value.trim() || undefined
    })
    blogs.value = result.items as Blog[]
    pagination.total = result.total
    // 删除末页最后一条后当前页可能越界，回退到最后一页重新拉取
    const totalPages = Math.max(1, Math.ceil(result.total / pagination.page_size))
    if (pagination.page > totalPages) {
      pagination.page = totalPages
      await loadBlogs()
      return
    }
  } catch (err) {
    console.error('Failed to load blogs', err)
    blogs.value = []
    pagination.total = 0
  } finally {
    loading.value = false
  }
}

function handlePageChange(page: number) {
  pagination.page = page
  loadBlogs()
}

function handlePageSizeChange(pageSize: number) {
  pagination.page_size = pageSize
  pagination.page = 1
  loadBlogs()
}

function handleStatusChange() {
  pagination.page = 1
  loadBlogs()
}

function handleSearch() {
  if (searchTimer) clearTimeout(searchTimer)
  searchTimer = setTimeout(() => {
    pagination.page = 1
    loadBlogs()
  }, 300)
}

function toDatetimeLocal(iso?: string | null) {
  if (!iso) return ''
  try {
    const d = new Date(iso)
    const pad = (n: number) => String(n).padStart(2, '0')
    return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`
  } catch {
    return ''
  }
}

function fromDatetimeLocal(value: string): number | undefined {
  if (!value) return 0 // 0 means clear
  const t = new Date(value)
  if (isNaN(t.getTime())) return undefined
  return Math.floor(t.getTime() / 1000)
}

function openCreate() {
  destroyEditor()
  editing.value = {
    title: '',
    content: '',
    summary: '',
    cover_image: '',
    status: 'draft',
    tags: '',
    publishedAtInput: ''
  }
  syncEditorLocale()
  // 切换文章时重建编辑器实例，避免复用旧实例导致内容错乱
  editorKey.value++
  coverUploadError.value = ''
}

function openEdit(row: Blog) {
  destroyEditor()
  editing.value = {
    id: row.id,
    title: row.title,
    content: row.content,
    summary: row.summary,
    cover_image: row.cover_image,
    status: row.status,
    tags: row.tags,
    publishedAtInput: toDatetimeLocal(row.published_at)
  }
  syncEditorLocale()
  editorKey.value++
  coverUploadError.value = ''
}

function closeDialog() {
  destroyEditor()
  editing.value = null
}

function syncEditorLocale() {
  i18nChangeLanguage(locale.value === 'en' ? 'en' : 'zh-CN')
}

/** 富文本空内容（<p><br></p>、&nbsp;）也视为空 */
function isContentEmpty(html: string): boolean {
  const text = html
    .replace(/<[^>]*>/g, '')
    .replace(/&nbsp;/gi, '')
    .replace(/&amp;/gi, '&')
    .trim()
  return text.length === 0 && !/<img\s/i.test(html)
}

async function saveBlog() {
  if (!editing.value) return
  if (!editing.value.title.trim() || isContentEmpty(editing.value.content)) {
    alert(t('admin.blog.titleContentRequired'))
    return
  }
  saving.value = true
  try {
    const publishedAt = fromDatetimeLocal(editing.value.publishedAtInput)
    const isCreate = !editing.value.id
    if (editing.value.id) {
      await adminBlogAPI.update(editing.value.id, {
        title: editing.value.title,
        content: editing.value.content,
        summary: editing.value.summary,
        cover_image: editing.value.cover_image,
        status: editing.value.status,
        tags: editing.value.tags,
        published_at: publishedAt
      })
    } else {
      await adminBlogAPI.create({
        title: editing.value.title,
        content: editing.value.content,
        summary: editing.value.summary,
        cover_image: editing.value.cover_image,
        status: editing.value.status,
        tags: editing.value.tags,
        published_at: publishedAt && publishedAt > 0 ? publishedAt : undefined
      })
    }
    closeDialog()
    // 新建的文章按 created_at 倒序排在首页，直接回到第 1 页便于确认
    if (isCreate) pagination.page = 1
    await loadBlogs()
  } catch (err: any) {
    console.error('Failed to save blog', err)
    alert(err?.message || t('admin.blog.saveFailed'))
  } finally {
    saving.value = false
  }
}

async function confirmDelete(row: Blog) {
  if (!confirm(t('admin.blog.confirmDelete', { title: row.title }))) return
  try {
    await adminBlogAPI.delete(row.id)
    await loadBlogs()
  } catch (err: any) {
    console.error('Failed to delete blog', err)
    alert(err?.message || t('admin.blog.deleteFailed'))
  }
}

onMounted(loadBlogs)

onBeforeUnmount(destroyEditor)
</script>

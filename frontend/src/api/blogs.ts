/**
 * Blog API endpoints
 */

import { apiClient } from './client'
import type { Blog, UserBlog, CreateBlogRequest, UpdateBlogRequest } from '@/types'

export interface ListBlogsParams {
  page?: number
  page_size?: number
  status?: 'draft' | 'published' | ''
  search?: string
  tag?: string
  sort_by?: string
  sort_order?: string
}

export interface ListBlogsResult {
  items: UserBlog[] | Blog[]
  total: number
  page: number
  page_size: number
}

/** 后台上传的图片（首页图片库/博客图片共用同一存储） */
export interface AdminUploadedImage {
  name: string
  url: string
  thumb_url: string
  size: number
  created_at: string
}

// 公开博客接口（无需认证）
export const publicBlogAPI = {
  async list(params: ListBlogsParams = {}): Promise<ListBlogsResult> {
    const { data } = await apiClient.get<ListBlogsResult>('/public/blogs', { params })
    return {
      items: data.items || [],
      total: data.total || 0,
      page: data.page || 1,
      page_size: data.page_size || 10
    }
  },
  async getByID(id: number): Promise<UserBlog> {
    const { data } = await apiClient.get<UserBlog>(`/public/blogs/${id}`)
    return data
  }
}

// 管理员博客接口
export const adminBlogAPI = {
  async list(params: ListBlogsParams = {}): Promise<ListBlogsResult> {
    const { data } = await apiClient.get<ListBlogsResult>('/admin/blogs', { params })
    return {
      items: data.items || [],
      total: data.total || 0,
      page: data.page || 1,
      page_size: data.page_size || 10
    }
  },
  async getByID(id: number): Promise<Blog> {
    const { data } = await apiClient.get<Blog>(`/admin/blogs/${id}`)
    return data
  },
  async create(req: CreateBlogRequest): Promise<Blog> {
    const { data } = await apiClient.post<Blog>('/admin/blogs', req)
    return data
  },
  async update(id: number, req: UpdateBlogRequest): Promise<Blog> {
    const { data } = await apiClient.put<Blog>(`/admin/blogs/${id}`, req)
    return data
  },
  async delete(id: number): Promise<{ message: string }> {
    const { data } = await apiClient.delete<{ message: string }>(`/admin/blogs/${id}`)
    return data
  },
  /**
   * Upload an image for blog cover / rich-text content.
   * Returns the public URL of the stored image.
   */
  async uploadImage(file: File): Promise<string> {
    const formData = new FormData()
    formData.append('file', file)
    const { data } = await apiClient.post<{ url: string }>('/admin/uploads/image', formData, {
      // Override the instance-level JSON content type; the browser appends the boundary.
      headers: { 'Content-Type': 'multipart/form-data' },
      timeout: 60000
    })
    return data?.url || ''
  },
  /** 列出已上传的图片（按上传时间倒序），首页图片库复用 */
  async listImages(limit = 100): Promise<{ items: AdminUploadedImage[]; total: number }> {
    const { data } = await apiClient.get<{ items: AdminUploadedImage[]; total: number }>(
      '/admin/uploads/images',
      { params: { limit } }
    )
    return { items: data.items || [], total: data.total || 0 }
  }
}

export default {
  public: publicBlogAPI,
  admin: adminBlogAPI
}

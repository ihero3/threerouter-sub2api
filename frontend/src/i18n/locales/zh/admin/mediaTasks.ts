export default {
  mediaTasks: {
    title: '媒体任务',
    description: '统一管理图片 / 视频 / 音频生成任务（统一媒体链路 media_tasks，新的生图与生视频任务都在这里）',
    userIdPlaceholder: '输入用户 ID 查询',
    emptyStateTitle: '请输入用户 ID',
    userIdRequired: '请在上方输入框中输入用户 ID，然后开始查询该用户的媒体任务。',
    noData: '暂无媒体任务',
    tryOtherFilters: '当前筛选条件下未找到媒体任务，请调整筛选条件。',
    loadFailed: '加载媒体任务失败',
    cancelSuccess: '任务已取消',
    cancelFailed: '取消任务失败',
    cancel: '取消',
    cancelConfirmTitle: '确认取消任务',
    cancelConfirmMessage: '确定要取消任务 #{id} 吗？取消后任务状态将被标记为 cancelled。',
    openMedia: '打开',
    finishedAt: '完成',
    status: {
      processing: '处理中',
      succeeded: '成功',
      failed: '失败',
      cancelled: '已取消'
    },
    columns: {
      taskId: '任务 ID',
      model: '模型',
      status: '状态',
      user: '用户',
      channel: '渠道',
      resolution: '规格',
      media: '产物',
      cost: '费用',
      error: '错误信息',
      createdAt: '创建时间'
    }
  }
}

export default {
  mediaTasks: {
    title: 'Media Tasks',
    description: 'Unified management of image / video / audio generation tasks (media_tasks table; all new image and video tasks land here)',
    userIdPlaceholder: 'Enter user ID to query',
    emptyStateTitle: 'Enter a user ID',
    userIdRequired: 'Enter a user ID above to start querying that user\'s media tasks.',
    noData: 'No media tasks',
    tryOtherFilters: 'No media tasks match the current filters. Try adjusting them.',
    loadFailed: 'Failed to load media tasks',
    cancelSuccess: 'Task cancelled',
    cancelFailed: 'Failed to cancel task',
    cancel: 'Cancel',
    cancelConfirmTitle: 'Cancel task',
    cancelConfirmMessage: 'Are you sure you want to cancel task #{id}? It will be marked as cancelled.',
    openMedia: 'Open',
    finishedAt: 'Finished',
    status: {
      processing: 'Processing',
      succeeded: 'Succeeded',
      failed: 'Failed',
      cancelled: 'Cancelled'
    },
    columns: {
      taskId: 'Task ID',
      model: 'Model',
      status: 'Status',
      user: 'User',
      channel: 'Channel',
      resolution: 'Spec',
      media: 'Output',
      cost: 'Cost',
      error: 'Error',
      createdAt: 'Created At'
    }
  }
}

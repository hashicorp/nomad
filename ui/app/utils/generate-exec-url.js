export default function generateExecUrl(router, { job, taskGroup, task, allocation }) {
  const queryParams = router.currentRoute.queryParams;

  if (task) {
    return router.urlFor('exec.task-group.task', job.plainId, taskGroup.name, task.name, {
      queryParams: {
        allocation: allocation.shortId,
        ...queryParams,
      },
    });
  } else if (taskGroup) {
    return router.urlFor('exec.task-group', job.plainId, taskGroup.name, { queryParams });
  } else if (allocation) {
    return router.urlFor('exec', job.plainId, { queryParams: { allocation: allocation.shortId, ...queryParams } });
  } else {
    return router.urlFor('exec', job.plainId, { queryParams });
  }
}

export default function generateExecUrl(router, { job, taskGroup, task, allocation }) {
  const queryParams = router.currentRoute.queryParams;

  if (task) {
    return router.urlFor('exec.task-group.task', job, taskGroup, task, {
      queryParams: {
        allocation,
        ...queryParams,
      },
    });
  } else if (taskGroup) {
    return router.urlFor('exec.task-group', job, taskGroup, { queryParams });
  } else if (allocation) {
    return router.urlFor('exec', job, { queryParams: { allocation, ...queryParams } });
  } else {
    return router.urlFor('exec', job, { queryParams });
  }
}

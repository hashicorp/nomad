export default function generateExecUrl(router, { job, taskGroup, task, allocation }) {
  if (task) {
    return router.urlFor('exec.task-group.task', job, taskGroup, task, {
      queryParams: {
        allocation,
      },
    });
  } else if (taskGroup) {
    return router.urlFor('exec.task-group', job, taskGroup);
  } else if (allocation) {
    return router.urlFor('exec', job, { queryParams: { allocation } });
  } else {
    return router.urlFor('exec', job);
  }
}

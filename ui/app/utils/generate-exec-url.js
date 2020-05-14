import { get } from '@ember/object';

export default function generateExecUrl(router, { job, taskGroup, task, allocation }) {
  const queryParams = router.currentRoute.queryParams;

  if (task) {
    return router.urlFor('exec.task-group.task', get(job, 'plainId'), get(taskGroup, 'name'), get(task, 'name'), {
      queryParams: {
        allocation: get(allocation, 'shortId'),
        ...queryParams,
      },
    });
  } else if (taskGroup) {
    return router.urlFor('exec.task-group', get(job, 'plainId'), get(taskGroup, 'name'), { queryParams });
  } else if (allocation) {
    return router.urlFor('exec', get(job, 'plainId'), { queryParams: { allocation: get(allocation, 'shortId'), ...queryParams } });
  } else {
    return router.urlFor('exec', get(job, 'plainId'), { queryParams });
  }
}

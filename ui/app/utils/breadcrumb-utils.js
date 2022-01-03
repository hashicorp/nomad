import PromiseObject from 'nomad-ui/utils/classes/promise-object';
import { qpBuilder } from 'nomad-ui/utils/classes/query-params';

export const jobCrumb = job => ({
  label: job.get('trimmedName'),
  args: [
    'jobs.job.index',
    job.get('plainId'),
    qpBuilder({
      jobNamespace: job.get('namespace.name') || 'default',
    }),
  ],
});

export const jobCrumbs = job => {
  if (!job) return [];

  if (job.get('parent.content')) {
    return [
      PromiseObject.create({
        promise: job.get('parent').then(parent => jobCrumb(parent)),
      }),
      jobCrumb(job),
    ];
  } else {
    return [jobCrumb(job)];
  }
};

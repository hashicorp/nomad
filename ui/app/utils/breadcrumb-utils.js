import PromiseObject from 'nomad-ui/utils/classes/promise-object';
import { qpBuilder } from 'nomad-ui/utils/classes/query-params';

export const jobCrumb = (job, isParameterizedOrPeriodic) => ({
  title: isParameterizedOrPeriodic ?? 'Job',
  label: job.get('trimmedName'),
  args: [
    'jobs.job.index',
    job.get('plainId'),
    qpBuilder({
      jobNamespace: job.get('namespace.name') || 'default',
    }),
  ],
});

const isParameterizedOrPeriodic = job =>
  job.get('periodic') ? 'Periodic Job' : job.get('parameterized') ? 'Dispatched Job' : '';

export const jobCrumbs = job => {
  if (!job) return [];

  if (job.get('parent.content')) {
    return [
      PromiseObject.create({
        promise: job.get('parent').then(parent => jobCrumb(parent)),
      }),
      jobCrumb(job, isParameterizedOrPeriodic(job)),
    ];
  } else {
    return [jobCrumb(job)];
  }
};

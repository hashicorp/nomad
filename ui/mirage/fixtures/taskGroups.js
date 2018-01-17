export default [
  {
    createAllocations: true,
    ephemeralDisk: {
      Sticky: false,
      SizeMB: 200,
      Migrate: true,
    },
    count: 3,
    name: 'fixture-array-g-0',
    jobId: 'job-100',
    taskIds: ['1000'],
    id: '1000',
    task_ids: ['1000'],
  },
];

import { Factory, faker, trait } from 'ember-cli-mirage';
import { provide } from '../utils';

const UUIDS = provide(100, faker.random.uuid.bind(faker.random));
const DEPLOYMENT_STATUSES = ['running', 'successful', 'paused', 'failed', 'cancelled'];

export default Factory.extend({
  id: i => (i / 100 >= 1 ? `${UUIDS[i]}-${i}` : UUIDS[i]),

  jobId: null,
  versionNumber: null,

  status: faker.list.random(...DEPLOYMENT_STATUSES),
  statusDescription: () => faker.lorem.sentence(),

  notActive: trait({
    status: faker.list.random(...DEPLOYMENT_STATUSES.without('running')),
  }),

  active: trait({
    status: 'running',
  }),

  afterCreate(deployment, server) {
    const job = server.db.jobs.find(deployment.jobId);
    const groups = job.taskGroupIds.map(id =>
      server.create('deployment-task-group-summary', {
        deployment,
        name: server.db.taskGroups.find(id).name,
        desiredCanaries: 1,
        promoted: false,
      })
    );

    deployment.update({
      deploymentTaskGroupSummaryIds: groups.mapBy('id'),
    });
  },
});

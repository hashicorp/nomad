import { Factory, faker } from 'ember-cli-mirage';

const TASK_STATUSES = ['pending', 'running', 'finished', 'failed'];
const REF_TIME = new Date();

export default Factory.extend({
  name: () => '!!!this should be set by the allocation that owns this task state!!!',
  state: faker.list.random(...TASK_STATUSES),
  startedAt: faker.date.past(2 / 365, REF_TIME),
  finishedAt() {
    if (['pending', 'running'].includes(this.state)) {
      return '0001-01-01T00:00:00Z';
    }
    return new Date(this.startedAt + Math.random(1000 * 60 * 3) + 50);
  },

  afterCreate(state, server) {
    const props = [
      'task-event',
      faker.random.number({ min: 1, max: 10 }),
      {
        taskStateId: state.id,
      },
    ].compact();

    const events = server.createList(...props);

    state.update({
      eventIds: events.mapBy('id'),
    });
  },
});

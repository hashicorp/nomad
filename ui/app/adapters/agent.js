import ApplicationAdapter from './application';

export default ApplicationAdapter.extend({
  pathForType: () => 'agent/members',
  urlForFindRecord() {
    const [, ...args] = arguments;
    return this.urlForFindAll(...args);
  },
});

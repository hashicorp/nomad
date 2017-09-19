import ApplicationAdapter from './application';

export default ApplicationAdapter.extend({
  findAllocations(node) {
    const url = `${this.buildURL('node', node.get('id'), node, 'findRecord')}/allocations`;
    return this.ajax(url, 'GET').then(allocs => {
      return this.store.pushPayload('allocation', {
        allocations: allocs,
      });
    });
  },
});

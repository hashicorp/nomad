import { attribute, clickable, collection } from 'ember-cli-page-object';

export default scope => ({
  scope,

  bars: collection('.bars > g', {
    id: attribute('data-test-client-status'),
    visit: clickable(),
  }),

  visitBar: async function(id) {
    const bar = this.bars.toArray().findBy('id', id);
    console.log('this.bars\n\n', bar.id);
    await bar.visit();
  },
});

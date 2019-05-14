import { inject as service } from '@ember/service';
import Component from '@ember/component';
import { lazyClick } from '../helpers/lazy-click';
import { watchRelationship } from 'nomad-ui/utils/properties/watch';
import WithVisibilityDetection from 'nomad-ui/mixins/with-component-visibility-detection';

export default Component.extend(WithVisibilityDetection, {
  store: service(),

  tagName: 'tr',
  classNames: ['client-node-row', 'is-interactive'],

  node: null,

  onClick() {},

  click(event) {
    lazyClick([this.onClick, event]);
  },

  didReceiveAttrs() {
    // Reload the node in order to get detail information
    const node = this.node;
    if (node) {
      node.reload().then(() => {
        this.watch.perform(node, 100);
      });
    }
  },

  visibilityHandler() {
    if (document.hidden) {
      this.watch.cancelAll();
    } else {
      const node = this.node;
      if (node) {
        this.watch.perform(node, 100);
      }
    }
  },

  willDestroy() {
    this.watch.cancelAll();
    this._super(...arguments);
  },

  watch: watchRelationship('allocations'),
});

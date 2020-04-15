import Component from '@ember/component';
import { inject as service } from '@ember/service';
import { task } from 'ember-concurrency';
import fetch from 'nomad-ui/utils/fetch';
import { getOwner } from '@ember/application';
import { bindKeyboardShortcuts, unbindKeyboardShortcuts } from 'ember-keyboard-shortcuts';

export default Component.extend({
  tagName: '',

  router: service(),
  store: service(),

  opened: false,

  keyboardShortcuts: {
    '/': {
      action: 'open',
      global: false,
      preventDefault: true,
    },
  },

  didInsertElement() {
    this._super(...arguments);
    bindKeyboardShortcuts(this);
  },

  willDestroyElement() {
    this._super(...arguments);
    unbindKeyboardShortcuts(this);
  },

  actions: {
    open() {
      if (this.select) {
        this.select.actions.open();
      }
    },

    storeSelect(select) {
      if (select) {
        this.select = select;
      }
    },

    select({ model }) {
      const itemModelName = model.constructor.modelName;

      if (itemModelName === 'job') {
        this.router.transitionTo('jobs.job', model.plainId);
      } else if (itemModelName === 'allocation') {
        this.router.transitionTo('allocations.allocation', model.id);
      } else if (itemModelName === 'node') {
        this.router.transitionTo('clients.client', model.id);
      }
    },
  },

  search: task(function*(prefix) {
    const applicationAdapter = getOwner(this).lookup('adapter:application');
    const searchUrl = applicationAdapter.urlForFindAll('job').replace('jobs', 'search');
    // FIXME hackery!

    const response = yield fetch(searchUrl, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({
        Prefix: prefix,
        Context: 'all',
      }),
    });
    const json = yield response.json();

    return Object.keys(json.Matches)
      .filter(key => json.Matches[key])
      .map(key => {
        return {
          groupName: key,
          options: collectModels(this.store, key, json.Matches[key]),
        };
      });
  }),
});

function collectModels(store, searchResultsTypeKey, matches) {
  console.log('type key', searchResultsTypeKey);
  if (searchResultsTypeKey === 'jobs') {
    return matches.map(id => {
      // FIXME don’t hardcode namespace
      const model = store.peekRecord('job', JSON.stringify([id, 'default']));
      return {
        model,
        label: model.name,
      };
    });
  } else if (searchResultsTypeKey === 'allocs') {
    return matches.map(id => {
      const model = store.peekRecord('allocation', id);
      return {
        model,
        label: model.id,
      };
    });
  } else if (searchResultsTypeKey === 'nodes') {
    return matches.map(id => {
      const model = store.peekRecord('node', id);
      return {
        model,
        label: model.id,
      };
    });
  }
}

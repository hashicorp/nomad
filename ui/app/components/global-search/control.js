import Component from '@ember/component';
import { inject as service } from '@ember/service';
import { task } from 'ember-concurrency';
import fetch from 'nomad-ui/utils/fetch';
import { getOwner } from '@ember/application';
import { bindKeyboardShortcuts, unbindKeyboardShortcuts } from 'ember-keyboard-shortcuts';

const SEARCH_PROPERTY_TO_LABEL = {
  allocs: 'Allocations',
  jobs: 'Jobs',
  nodes: 'Clients',
};

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

    async select({ model }) {
      const resolvedModel = await model.then();
      const itemModelName = resolvedModel.constructor.modelName;

      if (itemModelName === 'job') {
        this.router.transitionTo('jobs.job', resolvedModel.name);
      } else if (itemModelName === 'allocation') {
        this.router.transitionTo('allocations.allocation', resolvedModel.id);
      } else if (itemModelName === 'node') {
        this.router.transitionTo('clients.client', resolvedModel.id);
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

    return ['allocs', 'jobs', 'nodes']
      .filter(key => json.Matches[key])
      .map(key => {
        const matches = json.Matches[key];
        const label = `${SEARCH_PROPERTY_TO_LABEL[key]} (${matches.length})`;

        return {
          groupName: label,
          options: collectModels(this.store, key, json.Matches[key]),
        };
      });
  }),

  calculatePosition(trigger) {
    const { top, left, width } = trigger.getBoundingClientRect();
    return {
      style: {
        left,
        width,
        top,
      },
    };
  },
});

function collectModels(store, searchResultsTypeKey, matches) {
  if (searchResultsTypeKey === 'jobs') {
    return matches.map(id => {
      // FIXME don’t hardcode namespace
      const model = store.findRecord('job', JSON.stringify([id, 'default']));
      return {
        model,
        labelProperty: 'name',
        statusProperty: 'status',
        label: model.name,
      };
    });
  } else if (searchResultsTypeKey === 'allocs') {
    return matches.map(id => {
      const model = store.findRecord('allocation', id);
      return {
        model,
        labelProperty: 'id',
        statusProperty: 'clientStatus',
        label: model.id,
      };
    });
  } else if (searchResultsTypeKey === 'nodes') {
    return matches.map(id => {
      const model = store.findRecord('node', id);
      return {
        model,
        labelProperty: 'id',
        statusProperty: 'status',
        label: model.id,
      };
    });
  }
}

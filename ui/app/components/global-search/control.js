import Component from '@ember/component';
import { inject as service } from '@ember/service';
import { task } from 'ember-concurrency';
import fetch from 'nomad-ui/utils/fetch';
import { getOwner } from '@ember/application';
import { bindKeyboardShortcuts, unbindKeyboardShortcuts } from 'ember-keyboard-shortcuts';
import { run } from '@ember/runloop';

const SEARCH_PROPERTY_TO_LABEL = {
  allocs: 'Allocations',
  jobs: 'Jobs',
  nodes: 'Clients',
};

export default Component.extend({
  tagName: '',

  router: service(),
  store: service(),
  system: service(),

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

    openOnClickOrTab(select, { target }) {
      // Bypass having to press enter to access search after clicking/tabbing
      if (target.classList.contains('ember-power-select-trigger')) {
        run.next(() => {
          select.actions.open();
        });
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
    const query = new URLSearchParams();

    if (this.system.get('activeNamespace.id')) {
      query.append('namespace', this.system.activeNamespace.id);
    }

    if (this.system.activeRegion) {
      query.append('region', this.system.activeRegion);
    }

    const response = yield fetch(`${searchUrl}?${query}`, {
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
          options: collectModels(
            this.store,
            this.system.get('activeNamespace.id'),
            key,
            json.Matches[key]
          ),
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

function collectModels(store, namespace, searchResultsTypeKey, matches) {
  if (searchResultsTypeKey === 'jobs') {
    return matches.map(id => {
      const model = store.findRecord('job', JSON.stringify([id, namespace]));
      return {
        model,
        labelProperty: 'name',
        statusProperty: 'status',
      };
    });
  } else if (searchResultsTypeKey === 'allocs') {
    return matches.map(id => {
      const model = store.findRecord('allocation', id);
      return {
        model,
        labelProperty: 'id',
        statusProperty: 'clientStatus',
      };
    });
  } else if (searchResultsTypeKey === 'nodes') {
    return matches.map(id => {
      const model = store.findRecord('node', id);
      return {
        model,
        labelProperty: 'id',
        statusProperty: 'status',
      };
    });
  }
}

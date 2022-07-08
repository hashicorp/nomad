import Controller from '@ember/controller';
import { inject as service } from '@ember/service';
import { computed } from '@ember/object';
import { scheduleOnce } from '@ember/runloop';
import { serialize } from 'nomad-ui/utils/qp-serialize';
import { get, set } from '@ember/object';

export default class RunController extends Controller {
  @service router;
  @service system;
  @service store;

  queryParams = [
    {
      qpNamespace: 'namespace',
    },
  ];

  onSubmit(id, namespace) {
    this.router.transitionTo('jobs.job', `${id}@${namespace || 'default'}`);
  }
  @computed('qpNamespace')
  get optionsNamespaces() {
    const availableNamespaces = this.store
      .peekAll('namespace')
      .map((namespace) => ({
        key: namespace.name,
        label: namespace.name,
      }));

    availableNamespaces.unshift({
      key: '*',
      label: 'All (*)',
    });

    // Unset the namespace selection if it was server-side deleted
    if (!availableNamespaces.mapBy('key').includes(this.qpNamespace)) {
      scheduleOnce('actions', () => {
        // eslint-disable-next-line ember/no-side-effects
        set(this, 'qpNamespace', '*');
      });
    }

    return availableNamespaces;
  }

  setFacetQueryParam(queryParam, selection) {
    this.set(queryParam, serialize(selection));
    const model = get(this, 'model');
    set(
      model,
      'namespace',
      this.store.peekAll('namespace').find((ns) => ns.id === this.qpNamespace)
    );
  }
}

import { attr } from '@ember-data/model';
import Fragment from 'ember-data-model-fragments/fragment';
import { fragment } from 'ember-data-model-fragments/attributes';
import { computed } from '@ember/object';

export default class Service extends Fragment {
  @attr('string') name;
  @attr('string') portLabel;
  @attr() tags;
  @attr('string') onUpdate;
  @attr('string') provider;
  @fragment('consul-connect') connect;
  @attr() groupName;
  @attr() taskName;
  get refID() {
    return `${this.groupName || this.taskName}-${this.name}`;
  }
  @attr({ defaultValue: () => [] }) healthChecks;

  // TODO: find out why this doesnt update within the sidebar context
  @computed('healthChecks.[]')
  get mostRecentChecks() {
    console.log('mRC recompute');
    // Get unique check names, then get the most recent one
    return this.get('healthChecks')
      .mapBy('Check')
      .uniq()
      .map((name) => {
        // Assumtion: health checks are being pushed in sequential order (hence .reverse)
        return this.get('healthChecks')
          .reverse()
          .find((x) => x.Check === name);
      });
  }

  // TODO: make this compute on mostRecentChecks instead
  @computed('healthChecks.[]')
  get mostRecentCheckStatus() {
    // Get unique check names, then get the most recent one
    return this.get('healthChecks')
      .mapBy('Check')
      .uniq()
      .map((name) => {
        // Assumtion: health checks are being pushed in sequential order (hence .reverse)
        return this.get('healthChecks')
          .reverse()
          .find((x) => x.Check === name);
      })
      .mapBy('Status')
      .reduce((acc, curr) => {
        acc[curr] = (acc[curr] || 0) + 1;
        return acc;
      }, {});
  }
}

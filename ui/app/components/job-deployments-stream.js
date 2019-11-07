import Component from '@ember/component';
import { computed } from '@ember/object';
import { computed as overridable } from 'ember-overridable-computed';
import moment from 'moment';

export default Component.extend({
  tagName: 'ol',
  classNames: ['timeline'],

  deployments: overridable(() => []),

  sortedDeployments: computed('deployments.@each.versionSubmitTime', function() {
    return this.deployments
      .sortBy('versionSubmitTime')
      .reverse();
  }),

  annotatedDeployments: computed('sortedDeployments.@each.version', function() {
    const deployments = this.sortedDeployments;
    return deployments.map((deployment, index) => {
      const meta = {};

      if (index === 0) {
        meta.showDate = true;
      } else {
        const previousDeployment = deployments.objectAt(index - 1);
        const previousSubmitTime = previousDeployment.get('version.submitTime');
        const submitTime = deployment.get('submitTime');
        if (
          submitTime &&
          previousSubmitTime &&
          moment(previousSubmitTime)
            .startOf('day')
            .diff(moment(submitTime).startOf('day'), 'days') > 0
        ) {
          meta.showDate = true;
        }
      }

      return { deployment, meta };
    });
  }),
});

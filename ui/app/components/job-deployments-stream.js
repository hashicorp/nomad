import Ember from 'ember';
import moment from 'moment';

const { Component, computed } = Ember;

export default Component.extend({
  tagName: 'ol',
  classNames: ['timeline'],

  deployments: computed(() => []),

  sortedDeployments: computed('deployments.@each.version.submitTime', function() {
    return this.get('deployments')
      .sortBy('version.submitTime')
      .reverse();
  }),

  annotatedDeployments: computed('sortedDeployments.@each.version', function() {
    const deployments = this.get('sortedDeployments');
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

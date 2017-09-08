import Ember from 'ember';
import moment from 'moment';

const { Component, computed } = Ember;

export default Component.extend({
  tagName: 'ol',
  classNames: ['timeline'],

  deployments: computed(() => []),

  annotatedDeployments: computed('deployments.@each.version', function() {
    const deployments = this.get('deployments');
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

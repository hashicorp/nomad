import Component from '@ember/component';
import { computed } from '@ember/object';
import { computed as overridable } from 'ember-overridable-computed';
import moment from 'moment';

export default Component.extend({
  tagName: 'ol',
  classNames: ['timeline'],

  versions: overridable(() => []),

  // Passes through to the job-diff component
  verbose: true,

  annotatedVersions: computed('versions.[]', function() {
    const versions = this.versions
      .sortBy('submitTime')
      .reverse();
    return versions.map((version, index) => {
      const meta = {};

      if (index === 0) {
        meta.showDate = true;
      } else {
        const previousVersion = versions.objectAt(index - 1);
        const previousStart = moment(previousVersion.get('submitTime')).startOf('day');
        const currentStart = moment(version.get('submitTime')).startOf('day');
        if (previousStart.diff(currentStart, 'days') > 0) {
          meta.showDate = true;
        }
      }

      return { version, meta };
    });
  }),
});

import Ember from 'ember';
import moment from 'moment';

const { Component, computed } = Ember;

export default Component.extend({
  versions: computed(() => []),

  annotatedVersions: computed('versions.[]', function() {
    const versions = this.get('versions');
    return versions.map((version, index) => {
      const meta = {};

      if (index === 0) {
        meta.showDate = true;
      } else {
        const previousVersion = versions.objectAt(index - 1);
        if (
          moment(previousVersion.get('submitTime')).diff(
            moment(version.get('submitTime')),
            'days'
          ) > 0
        ) {
          meta.showDate = true;
        }
      }

      return { version, meta };
    });
  }),
});

import { text } from 'ember-cli-page-object';

import recommendationCard from 'nomad-ui/tests/pages/components/recommendation-card';

export default {
  group: text('[data-test-group]'),

  toggleButton: {
    scope: '.accordion-toggle',
  },

  card: recommendationCard,
};

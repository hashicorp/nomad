// @ts-check
import Service, { inject as service } from '@ember/service';
import { tracked } from '@glimmer/tracking';

export default class ThemeService extends Service {
  @service store;

  @tracked theme = null;

  constructor() {
    super(...arguments);
    this.loadTheme();
  }

  get bannerColor() {
    return this.theme.items['color-banner'];
  }

  async loadTheme() {
    console.log('loading theme');
    this.theme = await this.store.findRecord('variable', 'nomad/ui', {
      reload: true,
    });
    console.log('theme loaded', this.theme);
  }
}

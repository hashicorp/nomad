import Component from '@ember/component';
import classic from 'ember-classic-decorator';
import { inject as service } from '@ember/service';
import { attributeBindings } from '@ember-decorators/component';

@classic
@attributeBindings('data-test-global-header')
export default class GlobalHeader extends Component {
  @service config;
  @service system;
  @service token;
  @service router;

  'data-test-global-header' = true;
  onHamburgerClick() {}

  profileOptions = [
    {
      label: 'Authorization',
      key: 'authorization',
      action: () => {
        this.router.transitionTo('settings.tokens');
      },
    },
    {
      label: 'Sign Out',
      key: 'sign-out',
      action: () => {
        this.router.transitionTo('jobs.index');
      },
    },
  ];

  profileSelection = this.profileOptions[0];
}

import Component from '@ember/component';
import classic from 'ember-classic-decorator';
import { inject as service } from '@ember/service';

@classic
export default class GlobalHeader extends Component {
  @service config;
  @service system;

  'data-test-global-header' = true;
  onHamburgerClick() {}
}

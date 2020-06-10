import Component from '@ember/component';
import classic from 'ember-classic-decorator';

@classic
export default class GlobalHeader extends Component {
  'data-test-global-header' = true;
  onHamburgerClick() {}
}

import Fragment from 'ember-data-model-fragments/fragment';
import attr from 'ember-data/attr';

export default class SidecarProxyUpstream extends Fragment {
  @attr('string') destinationName;
  @attr('string') localBindPort;
}

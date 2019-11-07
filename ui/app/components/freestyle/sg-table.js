import Component from '@ember/component';
import { computed } from '@ember/object';
import productMetadata from 'nomad-ui/utils/styleguide/product-metadata';

export default Component.extend({
  searchTerm: '',

  currentPage: 1,
  sortProperty: 'name',
  sortDescending: false,

  shortList: productMetadata,

  longList: [
    { city: 'New York', growth: 0.048, population: '8405837', rank: '1', state: 'New York' },
    { city: 'Los Angeles', growth: 0.048, population: '3884307', rank: '2', state: 'California' },
    { city: 'Chicago', growth: -0.061, population: '2718782', rank: '3', state: 'Illinois' },
    { city: 'Houston', growth: 0.11, population: '2195914', rank: '4', state: 'Texas' },
    {
      city: 'Philadelphia',
      growth: 0.026,
      population: '1553165',
      rank: '5',
      state: 'Pennsylvania',
    },
    { city: 'Phoenix', growth: 0.14, population: '1513367', rank: '6', state: 'Arizona' },
    { city: 'San Antonio', growth: 0.21, population: '1409019', rank: '7', state: 'Texas' },
    { city: 'San Diego', growth: 0.105, population: '1355896', rank: '8', state: 'California' },
    { city: 'Dallas', growth: 0.056, population: '1257676', rank: '9', state: 'Texas' },
    { city: 'San Jose', growth: 0.105, population: '998537', rank: '10', state: 'California' },
    { city: 'Austin', growth: 0.317, population: '885400', rank: '11', state: 'Texas' },
    { city: 'Indianapolis', growth: 0.078, population: '843393', rank: '12', state: 'Indiana' },
    { city: 'Jacksonville', growth: 0.143, population: '842583', rank: '13', state: 'Florida' },
    {
      city: 'San Francisco',
      growth: 0.077,
      population: '837442',
      rank: '14',
      state: 'California',
    },
    { city: 'Columbus', growth: 0.148, population: '822553', rank: '15', state: 'Ohio' },
    {
      city: 'Charlotte',
      growth: 0.391,
      population: '792862',
      rank: '16',
      state: 'North Carolina',
    },
    { city: 'Fort Worth', growth: 0.451, population: '792727', rank: '17', state: 'Texas' },
    { city: 'Detroit', growth: -0.271, population: '688701', rank: '18', state: 'Michigan' },
    { city: 'El Paso', growth: 0.194, population: '674433', rank: '19', state: 'Texas' },
    { city: 'Memphis', growth: -0.053, population: '653450', rank: '20', state: 'Tennessee' },
    { city: 'Seattle', growth: 0.156, population: '652405', rank: '21', state: 'Washington' },
    { city: 'Denver', growth: 0.167, population: '649495', rank: '22', state: 'Colorado' },
    {
      city: 'Washington',
      growth: 0.13,
      population: '646449',
      rank: '23',
      state: 'District of Columbia',
    },
    { city: 'Boston', growth: 0.094, population: '645966', rank: '24', state: 'Massachusetts' },
    {
      city: 'Nashville-Davidson',
      growth: 0.162,
      population: '634464',
      rank: '25',
      state: 'Tennessee',
    },
    { city: 'Baltimore', growth: -0.04, population: '622104', rank: '26', state: 'Maryland' },
    { city: 'Oklahoma City', growth: 0.202, population: '610613', rank: '27', state: 'Oklahoma' },
    {
      city: 'Louisville/Jefferson County',
      growth: 0.1,
      population: '609893',
      rank: '28',
      state: 'Kentucky',
    },
    { city: 'Portland', growth: 0.15, population: '609456', rank: '29', state: 'Oregon' },
    { city: 'Las Vegas', growth: 0.245, population: '603488', rank: '30', state: 'Nevada' },
    { city: 'Milwaukee', growth: 0.003, population: '599164', rank: '31', state: 'Wisconsin' },
    { city: 'Albuquerque', growth: 0.235, population: '556495', rank: '32', state: 'New Mexico' },
    { city: 'Tucson', growth: 0.075, population: '526116', rank: '33', state: 'Arizona' },
    { city: 'Fresno', growth: 0.183, population: '509924', rank: '34', state: 'California' },
    { city: 'Sacramento', growth: 0.172, population: '479686', rank: '35', state: 'California' },
    { city: 'Long Beach', growth: 0.015, population: '469428', rank: '36', state: 'California' },
    { city: 'Kansas City', growth: 0.055, population: '467007', rank: '37', state: 'Missouri' },
    { city: 'Mesa', growth: 0.135, population: '457587', rank: '38', state: 'Arizona' },
    { city: 'Virginia Beach', growth: 0.051, population: '448479', rank: '39', state: 'Virginia' },
    { city: 'Atlanta', growth: 0.062, population: '447841', rank: '40', state: 'Georgia' },
    {
      city: 'Colorado Springs',
      growth: 0.214,
      population: '439886',
      rank: '41',
      state: 'Colorado',
    },
    { city: 'Omaha', growth: 0.059, population: '434353', rank: '42', state: 'Nebraska' },
    { city: 'Raleigh', growth: 0.487, population: '431746', rank: '43', state: 'North Carolina' },
    { city: 'Miami', growth: 0.149, population: '417650', rank: '44', state: 'Florida' },
    { city: 'Oakland', growth: 0.013, population: '406253', rank: '45', state: 'California' },
    { city: 'Minneapolis', growth: 0.045, population: '400070', rank: '46', state: 'Minnesota' },
    { city: 'Tulsa', growth: 0.013, population: '398121', rank: '47', state: 'Oklahoma' },
    { city: 'Cleveland', growth: -0.181, population: '390113', rank: '48', state: 'Ohio' },
    { city: 'Wichita', growth: 0.097, population: '386552', rank: '49', state: 'Kansas' },
    { city: 'Arlington', growth: 0.133, population: '379577', rank: '50', state: 'Texas' },
  ],

  filteredShortList: computed('searchTerm', 'shortList.[]', function() {
    const term = this.searchTerm.toLowerCase();
    return this.shortList.filter(product => product.name.toLowerCase().includes(term));
  }),

  sortedShortList: computed('shortList.[]', 'sortProperty', 'sortDescending', function() {
    const sorted = this.shortList.sortBy(this.sortProperty);
    return this.sortDescending ? sorted.reverse() : sorted;
  }),
});

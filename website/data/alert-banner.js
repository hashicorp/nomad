export const ALERT_BANNER_ACTIVE = true

// https://github.com/hashicorp/web-components/tree/master/packages/alert-banner
export default {
  linkText: 'Register now',
  url: 'https://www.hashicorp.com/events/webinars/introducing-nomad-1-0',
  tag: 'ANNOUNCEMENT',
  text:
    'Join us on Oct 27th for the Nomad 1.0 Product Announcement with Armon Dadgar',
  // Set the `expirationDate prop with a datetime string (e.g. `2020-01-31T12:00:00-07:00`)
  // if you'd like the component to stop showing at or after a certain date
  expirationDate: '2020-10-27T09:00:00-07:00',
}

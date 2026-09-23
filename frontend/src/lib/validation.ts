export function isValidEmail(value: string): boolean {
  return /^[A-Za-z0-9.!#$%&'*+\/=^_`{|}~-]+@[A-Za-z0-9](?:[A-Za-z0-9-]*[A-Za-z0-9])?(?:\.[A-Za-z0-9](?:[A-Za-z0-9-]*[A-Za-z0-9])?)+$/.test(value.trim());
}

export function isValidPhone(value: string): boolean {
  const phone = value.trim();
  const digits = phone.replace(/[^0-9]/g, '');
  return /^\+?[0-9][0-9 ()-]*[0-9]$/.test(phone) && digits.length >= 7 && digits.length <= 15;
}

export function isValidPlaceName(value: string): boolean {
  return /^[\p{L}]+(?:[ .'-][\p{L}]+)*$/u.test(value.trim());
}

export function isValidPostalCode(value: string): boolean {
  return /^[A-Za-z0-9][A-Za-z0-9 -]{1,10}[A-Za-z0-9]$/.test(value.trim());
}

export function isValidCountryCode(value: string): boolean {
  return /^[A-Za-z]{2}$/.test(value.trim());
}
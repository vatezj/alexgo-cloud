import dayjs from 'dayjs';

/** MySQL datetime / ISO 时间串统一展示为 YYYY-MM-DD HH:mm:ss。 */
export function formatDateTime(value?: null | Date | number | string): string {
  if (value === null || value === undefined || value === '') return '';
  const d = dayjs(value);
  return d.isValid() ? d.format('YYYY-MM-DD HH:mm:ss') : String(value);
}

import { Router } from 'express';
import { getZiweiPalaces } from '../services/ziwei.js';
import { getQimenPalaces } from '../services/qimen.js';
import { scoreSeats } from '../services/scoring.js';

const router = Router();

router.post('/calc', (req, res) => {
  try {
    const { birthDate, birthHourIndex, gender, playTime } = req.body ?? {};

    if (!birthDate || birthHourIndex == null || !gender || !playTime) {
      return res.status(400).json({ error: '缺少必要参数' });
    }
    if (!Number.isInteger(birthHourIndex) || birthHourIndex < 0 || birthHourIndex > 12) {
      return res.status(400).json({ error: 'birthHourIndex 须为 0–12 的整数' });
    }
    if (gender !== 'male' && gender !== 'female') {
      return res.status(400).json({ error: 'gender 须为 male 或 female' });
    }

    const ziwei = getZiweiPalaces({ birthDate, birthHourIndex, gender });
    const qimen = getQimenPalaces({ playTime });
    const recommendations = scoreSeats(qimen, ziwei).map((r, i) => ({
      rank: i + 1,
      ...r,
    }));

    res.json({
      recommendations,
      meta: {
        qimenConvention: '时家奇门·拆补法·转盘·中五寄坤',
        dayBoundary: '23:00子初换日',
        trueSolarTime: false,
      },
      disclaimer: '仅供娱乐参考，请勿沉迷赌博。',
    });
  } catch (err) {
    console.error(err);
    res.status(500).json({ error: '排盘计算失败，请检查输入时间格式' });
  }
});

export default router;

import express from 'express';
import path from 'path';
import { createServer as createViteServer } from 'vite';
import YahooFinance from 'yahoo-finance2';
import cors from 'cors';

const yahooFinance = new YahooFinance();

async function startServer() {
  const app = express();
  const PORT = 3000;

  app.use(cors());
  app.use(express.json());

  // API to fetch asset price
  app.get('/api/exchange/:pair', async (req, res) => {
    try {
      const { pair } = req.params; // e.g. USDCNY
      const quote = await yahooFinance.quote(`${pair}=X`);
      if (quote && quote.regularMarketPrice) {
        res.json({ rate: quote.regularMarketPrice });
      } else {
        res.status(404).json({ error: 'Rate not found' });
      }
    } catch (e) {
      res.status(500).json({ error: 'Failed to fetch exchange rate' });
    }
  });

  app.get('/api/price/:symbol', async (req, res) => {
    try {
      const { symbol } = req.params;
      if (!symbol) {
        return res.status(400).json({ error: 'Symbol is required' });
      }

      let querySymbol = symbol.toUpperCase();
      
      // Auto-detect A-shares 6-digit codes and apply Yahoo Finance suffixes
      if (/^\d{6}$/.test(querySymbol)) {
        if (/^(6|5)\d{5}$/.test(querySymbol)) {
          querySymbol += '.SS'; // Shanghai
        } else if (/^(0|3|1|2)\d{5}$/.test(querySymbol)) {
          querySymbol += '.SZ'; // Shenzhen
        }
      } else if (/^SH\d{6}$/i.test(symbol)) {
        querySymbol = symbol.slice(2).toUpperCase() + '.SS';
      } else if (/^SZ\d{6}$/i.test(symbol)) {
        querySymbol = symbol.slice(2).toUpperCase() + '.SZ';
      }

      const quote = await yahooFinance.quote(querySymbol, {
        lang: 'zh-CN',
        region: 'CN'
      });
      if (!quote || !quote.regularMarketPrice) {
        return res.status(404).json({ error: 'Price not found' });
      }

      let price = quote.regularMarketPrice;
      const currency = quote.currency || 'USD';
      
      let targetCurrency = 'CNY'; // Hardcode targeting CNY
      
      // Attempt currency conversion if it's not CNY
      if (currency !== targetCurrency) {
        // e.g. "USDCNY=X"
        const fxSymbol = `${currency}${targetCurrency}=X`;
        try {
          const fxQuote = await yahooFinance.quote(fxSymbol);
          if (fxQuote && fxQuote.regularMarketPrice) {
            price = price * fxQuote.regularMarketPrice;
          }
        } catch (fxErr) {
          console.error(`FX fetch error for ${fxSymbol}:`, fxErr);
          // Fall back to original price if FX fails, though probably we should inform client
        }
      }

      res.json({
        symbol: quote.symbol,
        name: quote.shortName || quote.longName,
        price: price,           // Converted price
        originalPrice: quote.regularMarketPrice,
        currency: targetCurrency,
        originalCurrency: currency
      });
      
    } catch (e) {
      console.error('Yahoo Finance Error:', e);
      res.status(500).json({ error: 'Failed to fetch price' });
    }
  });

  // Vite middleware for development
  if (process.env.NODE_ENV !== 'production') {
    const vite = await createViteServer({
      server: { middlewareMode: true },
      appType: 'spa',
    });
    app.use(vite.middlewares);
  } else {
    const distPath = path.join(process.cwd(), 'dist');
    app.use(express.static(distPath));
    app.get('*', (req, res) => {
      res.sendFile(path.join(distPath, 'index.html'));
    });
  }

  app.listen(PORT, '0.0.0.0', () => {
    console.log(`Server running on http://localhost:${PORT}`);
  });
}

startServer();

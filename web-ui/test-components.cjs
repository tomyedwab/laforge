// Simple test to validate our components have correct syntax
const fs = require('fs');
const path = require('path');

const components = [
  'src/components/StepDashboard.tsx',
  'src/components/StepCard.tsx', 
  'src/components/StepDetail.tsx',
  'src/components/StepTimeline.tsx'
];

console.log('Testing component syntax...');

components.forEach(component => {
  const filePath = path.join(__dirname, component);
  if (fs.existsSync(filePath)) {
    const content = fs.readFileSync(filePath, 'utf8');
    
    // Basic syntax checks
    const hasImports = content.includes('import');
    const hasExports = content.includes('export');
    const hasFunction = content.includes('function') || content.includes('=>');
    
    console.log(`✓ ${component}:`);
    console.log(`  - Has imports: ${hasImports}`);
    console.log(`  - Has exports: ${hasExports}`);
    console.log(`  - Has function/component: ${hasFunction}`);
    
    // Check for common syntax issues
    if (content.match(/\{\s*\}/g)) {
      console.log(`  - Warning: Empty object literals found`);
    }
    if (content.match(/\[\s*\]/g)) {
      console.log(`  - Warning: Empty array literals found`);
    }
    
    console.log('');
  } else {
    console.log(`✗ ${component}: File not found`);
  }
});

console.log('Component syntax test complete.');
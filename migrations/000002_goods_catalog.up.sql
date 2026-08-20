-- WOAson — только физические товары. Классифайд (авто, недвижимость, услуги) снимаем с витрины.
DELETE FROM products
WHERE category NOT IN (
    'odezhda', 'obuv', 'aksessuary', 'ukrasheniya',
    'elektronika', 'bytovaya', 'kompjutery', 'igry',
    'dom', 'mebel', 'kuhnya', 'sad', 'remont',
    'krasota', 'zdorovie',
    'detiam', 'zootovary',
    'sport', 'hobbi', 'knigi', 'kantselyariya',
    'produkty'
);

ALTER TABLE products
    DROP CONSTRAINT IF EXISTS products_goods_category_chk;

ALTER TABLE products
    ADD CONSTRAINT products_goods_category_chk CHECK (category IN (
        'odezhda', 'obuv', 'aksessuary', 'ukrasheniya',
        'elektronika', 'bytovaya', 'kompjutery', 'igry',
        'dom', 'mebel', 'kuhnya', 'sad', 'remont',
        'krasota', 'zdorovie',
        'detiam', 'zootovary',
        'sport', 'hobbi', 'knigi', 'kantselyariya',
        'produkty'
    ));

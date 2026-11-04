-- Dev data: 8 demo teachers in Almaty (users 1001-1008), all live with a running plan.
-- Usage: psql -v admin_id=<telegram_id> ... < dev.sql   (make seed ADMIN=<id>)
-- Re-running replaces the demo profiles.
\set ON_ERROR_STOP on
BEGIN;

INSERT INTO users (telegram_id, name, username, role) VALUES (:admin_id, 'Admin', NULL, 'admin')
ON CONFLICT (telegram_id) DO UPDATE SET role = 'admin';

INSERT INTO users (telegram_id, name, username) VALUES
    (1001, 'Айгерим Сапарова', 'aigerim_math'),
    (1002, 'Ерлан Ахметов', 'erlan_physics'),
    (1003, 'Anna Kim', 'anna_ielts'),
    (1004, 'Динара Омарова', 'dinara_piano'),
    (1005, 'Тимур Нургалиев', 'timur_code'),
    (1006, 'Сауле Жуматова', 'saule_kazakh'),
    (1007, 'Марат Исаев', 'marat_admission'),
    (1008, 'Ольга Петрова', 'olga_chem')
ON CONFLICT (telegram_id) DO NOTHING;

DELETE FROM specialists WHERE user_id BETWEEN 1001 AND 1008;

INSERT INTO specialists (user_id, kind, status, display_name, bio, education, experience_years, verified,
                         city, district, address, formats, languages, trial_kind, trial_price, lesson_minutes,
                         submitted_at, approved_at)
VALUES
    (1001, 'teacher', 'live', 'Айгерим Сапарова', 'Готовлю к ЕНТ по математике и мат. грамотности. Средний балл учеников — 35 из 40. Объясняю просто, даю много практики.', 'КазНУ им. аль-Фараби, механико-математический факультет', 8, true, 'almaty', 'bostandyk', 'ул. Тимирязева 42', '{online,teacher_place}', '{ru,kk}', 'free', NULL, 60, now(), now() - interval '20 days'),
    (1002, 'teacher', 'live', 'Ерлан Ахметов', 'Физика для 7–11 классов, подготовка к олимпиадам и ЕНТ. Решаем задачи, разбираем теорию на понятных примерах.', 'КазНТУ им. Сатпаева', 12, false, 'almaty', 'almaly', 'пр. Абая 15', '{online,student_home}', '{ru}', 'paid', 2000, 90, now(), now() - interval '15 days'),
    (1003, 'teacher', 'live', 'Anna Kim', 'IELTS 8.0 certified teacher. Speaking and writing focus, mock tests every month. Уроки на русском или английском.', 'KIMEP University, CELTA', 6, true, 'almaty', 'medeu', '', '{online}', '{en,ru}', 'free', NULL, 60, now(), now() - interval '10 days'),
    (1004, 'teacher', 'live', 'Динара Омарова', 'Фортепиано для детей с 5 лет и взрослых. Классика и современная музыка, подготовка к поступлению в музыкальную школу.', 'Казахская национальная консерватория им. Курмангазы', 15, false, 'almaty', 'auezov', 'мкр. Жетысу-2, 14', '{teacher_place,student_home}', '{ru,kk}', 'none', NULL, 45, now(), now() - interval '9 days'),
    (1005, 'teacher', 'live', 'Тимур Нургалиев', 'Python и веб-разработка для школьников и взрослых. Делаем реальные проекты: боты, сайты, игры.', 'Astana IT University', 4, false, 'almaty', 'bostandyk', '', '{online}', '{ru,kk,en}', 'free', NULL, 60, now(), now() - interval '5 days'),
    (1006, 'teacher', 'live', 'Сауле Жуматова', 'Қазақ тілі мен әдебиеті, ҰБТ-ға дайындық. Казахский язык для взрослых с нуля — разговорная практика.', 'ҚазҰПУ им. Абая', 20, true, 'almaty', 'turksib', 'ул. Сейфуллина 100', '{online,teacher_place}', '{kk,ru}', 'free', NULL, 60, now(), now() - interval '3 days'),
    (1007, 'consultant', 'live', 'Марат Исаев', 'Консультации по поступлению в NU, KBTU и зарубежные вузы: выбор вуза, мотивационное письмо, стратегия подготовки.', 'Nazarbayev University, MBA', 7, true, 'almaty', 'medeu', '', '{online}', '{ru,en}', 'paid', 5000, 60, now(), now() - interval '2 days'),
    (1008, 'teacher', 'live', 'Ольга Петрова', 'Химия и биология для школьников, подготовка к ЕНТ медицинского профиля.', 'КазНМУ им. Асфендиярова', 10, false, 'almaty', 'zhetysu', 'ул. Кабдолова 8', '{teacher_place,online}', '{ru}', 'none', NULL, 60, now(), now() - interval '1 day');

INSERT INTO specialist_subjects (specialist_id, subject_id, levels, price_kzt)
SELECT s.id, sj.id, x.levels::text[], x.price
FROM (VALUES
    (1001, 'ent-math', '{high,ent}', 6000), (1001, 'ent-math-literacy', '{ent}', 6000), (1001, 'algebra', '{middle,high}', 5000),
    (1002, 'physics', '{middle,high,ent}', 7000),
    (1003, 'ielts', '{high,university,adult}', 12000), (1003, 'english', '{middle,high,adult}', 8000),
    (1004, 'piano', '{primary,middle,adult}', 7000),
    (1005, 'python', '{middle,high,adult}', 9000), (1005, 'coding-for-kids', '{primary,middle}', 6000),
    (1006, 'kazakh-language', '{middle,high,ent}', 5000), (1006, 'kazakh-for-adults', '{adult}', 6000),
    (1007, 'admission-kz', '{high}', 15000), (1007, 'study-abroad', '{high,university}', 20000),
    (1008, 'chemistry', '{middle,high,ent}', 6000), (1008, 'biology', '{middle,high,ent}', 6000)
) AS x(user_id, slug, levels, price)
JOIN specialists s ON s.user_id = x.user_id
JOIN subjects sj ON sj.slug = x.slug;

INSERT INTO subscriptions (specialist_id, plan, starts_at, ends_at, note)
SELECT s.id,
       CASE WHEN s.user_id IN (1001, 1003) THEN 'featured' WHEN s.user_id = 1008 THEN 'trial' ELSE 'founding' END,
       now() - interval '1 day', now() + interval '60 days', 'dev seed'
FROM specialists s WHERE s.user_id BETWEEN 1001 AND 1008;

-- Weekdays 15:00-21:00 and Saturday 10:00-14:00 for everyone.
INSERT INTO availability_rules (specialist_id, weekday, starts_at, ends_at)
SELECT s.id, wd, '15:00'::time, '21:00'::time FROM specialists s, generate_series(1, 5) wd WHERE s.user_id BETWEEN 1001 AND 1008
UNION ALL
SELECT s.id, 6, '10:00'::time, '14:00'::time FROM specialists s WHERE s.user_id BETWEEN 1001 AND 1008;

COMMIT;

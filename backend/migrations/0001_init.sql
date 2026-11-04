-- +goose Up
CREATE EXTENSION IF NOT EXISTS btree_gist;
CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE TABLE users (
    telegram_id   bigint PRIMARY KEY,
    name          text NOT NULL DEFAULT '',
    username      text,
    language_code text,
    role          text NOT NULL DEFAULT 'user' CHECK (role IN ('user', 'admin')),
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE subjects (
    id       bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    slug     text NOT NULL UNIQUE CHECK (slug ~ '^[a-z0-9]+(-[a-z0-9]+)*$'),
    name_ru  text NOT NULL,
    name_kk  text NOT NULL,
    name_en  text NOT NULL,
    category text NOT NULL CHECK (category IN ('school', 'exam', 'language', 'it', 'music', 'arts', 'consulting', 'other')),
    position int NOT NULL DEFAULT 0
);

INSERT INTO subjects (slug, name_ru, name_kk, name_en, category, position) VALUES
    ('math',               'Математика',                        'Математика',                       'Mathematics',                 'school', 10),
    ('algebra',            'Алгебра',                           'Алгебра',                          'Algebra',                     'school', 11),
    ('geometry',           'Геометрия',                         'Геометрия',                        'Geometry',                    'school', 12),
    ('physics',            'Физика',                            'Физика',                           'Physics',                     'school', 13),
    ('chemistry',          'Химия',                             'Химия',                            'Chemistry',                   'school', 14),
    ('biology',            'Биология',                          'Биология',                         'Biology',                     'school', 15),
    ('geography',          'География',                         'География',                        'Geography',                   'school', 16),
    ('history-kz',         'История Казахстана',                'Қазақстан тарихы',                 'History of Kazakhstan',       'school', 17),
    ('world-history',      'Всемирная история',                 'Дүниежүзі тарихы',                 'World history',               'school', 18),
    ('kazakh-language',    'Казахский язык',                    'Қазақ тілі',                       'Kazakh language',             'school', 19),
    ('kazakh-literature',  'Казахская литература',              'Қазақ әдебиеті',                   'Kazakh literature',           'school', 20),
    ('russian-language',   'Русский язык',                      'Орыс тілі',                        'Russian language',            'school', 21),
    ('russian-literature', 'Русская литература',                'Орыс әдебиеті',                    'Russian literature',          'school', 22),
    ('informatics',        'Информатика',                       'Информатика',                      'Computer science',            'school', 23),
    ('primary-school',     'Начальная школа',                   'Бастауыш сынып',                   'Primary school',              'school', 24),
    ('law-basics',         'Основы права',                      'Құқық негіздері',                  'Law basics',                  'school', 25),
    ('ent-math',           'ЕНТ: математика',                   'ҰБТ: математика',                  'UNT: mathematics',            'exam',   30),
    ('ent-math-literacy',  'ЕНТ: математическая грамотность',   'ҰБТ: математикалық сауаттылық',    'UNT: math literacy',          'exam',   31),
    ('ent-reading',        'ЕНТ: грамотность чтения',           'ҰБТ: оқу сауаттылығы',             'UNT: reading literacy',       'exam',   32),
    ('ent-history',        'ЕНТ: история Казахстана',           'ҰБТ: Қазақстан тарихы',            'UNT: history of Kazakhstan',  'exam',   33),
    ('nis-prep',           'Подготовка в НИШ',                  'НЗМ-ге дайындық',                  'NIS entrance prep',           'exam',   34),
    ('ielts',              'IELTS',                             'IELTS',                            'IELTS',                       'exam',   35),
    ('toefl',              'TOEFL',                             'TOEFL',                            'TOEFL',                       'exam',   36),
    ('sat',                'SAT',                               'SAT',                              'SAT',                         'exam',   37),
    ('nuet',               'NUET',                              'NUET',                             'NUET',                        'exam',   38),
    ('english',            'Английский язык',                   'Ағылшын тілі',                     'English',                     'language', 50),
    ('german',             'Немецкий язык',                     'Неміс тілі',                       'German',                      'language', 51),
    ('french',             'Французский язык',                  'Француз тілі',                     'French',                      'language', 52),
    ('chinese',            'Китайский язык',                    'Қытай тілі',                       'Chinese',                     'language', 53),
    ('korean',             'Корейский язык',                    'Корей тілі',                       'Korean',                      'language', 54),
    ('turkish',            'Турецкий язык',                     'Түрік тілі',                       'Turkish',                     'language', 55),
    ('spanish',            'Испанский язык',                    'Испан тілі',                       'Spanish',                     'language', 56),
    ('kazakh-for-adults',  'Казахский для взрослых',            'Ересектерге қазақ тілі',           'Kazakh for adults',           'language', 57),
    ('python',             'Программирование на Python',        'Python бағдарламалау',             'Python programming',          'it',     70),
    ('web-development',    'Веб-разработка',                    'Веб-әзірлеу',                      'Web development',             'it',     71),
    ('coding-for-kids',    'Программирование для детей',        'Балаларға бағдарламалау',          'Coding for kids',             'it',     72),
    ('piano',              'Фортепиано',                        'Фортепиано',                       'Piano',                       'music',  80),
    ('guitar',             'Гитара',                            'Гитара',                           'Guitar',                      'music',  81),
    ('vocals',             'Вокал',                             'Вокал',                            'Singing',                     'music',  82),
    ('dombra',             'Домбра',                            'Домбыра',                          'Dombra',                      'music',  83),
    ('drawing',            'Рисование',                         'Сурет салу',                       'Drawing',                     'arts',   90),
    ('chess',              'Шахматы',                           'Шахмат',                           'Chess',                       'arts',   91),
    ('speech-therapy',     'Логопед',                           'Логопед',                          'Speech therapy',              'other',  110),
    ('admission-kz',       'Поступление в вузы Казахстана',     'ҚР жоғары оқу орындарына түсу',    'University admission (KZ)',   'consulting', 100),
    ('study-abroad',       'Учёба за рубежом',                  'Шетелде оқу',                      'Study abroad',                'consulting', 101),
    ('career',             'Профориентация и карьера',          'Кәсіптік бағдар және мансап',      'Career guidance',             'consulting', 102),
    ('exam-strategy',      'Стратегия подготовки к экзаменам',  'Емтиханға дайындық стратегиясы',   'Exam strategy',               'consulting', 103);

-- +goose Down
DROP TABLE subjects;
DROP TABLE users;

from bs4 import BeautifulSoup
import gevent
from gevent.queue import Queue
from gevent import monkey
import requests
from time import time
from pprint import pprint
from StringIO import StringIO
import zipfile
import argparse

monkey.patch_socket()


sites = {
    'mangareader': {
        'url': 'http://www.mangareader.net',
        'page_list': lambda p: ['http://www.mangareader.net' + x['value'] for x in p.find_all('option')],
        'img': lambda p: p.find(id='img').get('src')
    }
}


class Gevent_queue:
    '''
    Generic queue using gevent
        tasks: list of tasks to be executed
        worker_function: the function that executes a task
    Each worker will execute `worker_function(task)`
    >>> q = Gevent_queue(tasks, worker_function)
    >>> q.execute()
    '''
    def __init__(self, tasks, worker_func, workers=4):
        self.queue = Queue()
        self.tasks = tasks  # list of tasks
        self.workers = workers  # number of workers
        self.out = []  # list of results
        self.worker_func = worker_func

    def worker(self, n):
        while not self.queue.empty():
            start_time = time()
            task = self.queue.get()

            # execute worker function
            worker_out = self.worker_func(task)
            self.out.append(worker_out)

            # print 'Worker {0} finished {1} in {2}'.format(n, task, time()-start_time)

    def execute(self):
        for task in self.tasks:
            self.queue.put_nowait(task)
        gevent.joinall([gevent.spawn(self.worker, x) for x in xrange(self.workers)])
        self.out.sort()
        return self.out


class Download:
    '''
    Download single chapter
    >>> d = Download(sites['mangareader'], '/naruto/2')
    >>> d.execute()
    '''
    def __init__(self, site, path):
        self.site = site
        self.path = path
        self.filename = path.replace('/', '')

    def execute(self):
        def worker_func(task):
            '''
            task is of the form (pagenum, pageurl)
            '''
            # get page
            res_raw = requests.get(task[1])
            res = BeautifulSoup(res_raw.text, 'html.parser')
            img = self.site['img'](res)

            # get img
            jpg_raw = requests.get(img)

            # finish task
            return (task[0], img, jpg_raw.content)

        # get page 1 + page 1 img + links to other pages
        print 'Getting page list for', self.path
        page_raw = requests.get(self.site['url'] + self.path)
        page = BeautifulSoup(page_raw.text, 'html.parser')
        img1 = self.site['img'](page)
        jpg_raw = requests.get(img1)

        # get list of pages
        pages = self.site['page_list'](page)

        # tasks format is [(pagenum, pageurl)]
        tasks = [(i+1,p) for i, p in enumerate(pages[1:])]

        # get multiple pages
        start_time = time()
        q = Gevent_queue(tasks, worker_func=worker_func, workers=4)
        q_out = q.execute()

        # save into a cbz
        with zipfile.ZipFile(self.filename + '.cbz', 'w', zipfile.ZIP_STORED) as cbz:
            cbz.writestr(self.filename + '-0.jpg', jpg_raw.content)
            for img in q_out:
                cbz.writestr(self.filename + '-' + str(img[0]) + '.jpg', img[2])

        print '>>>', self.path, 'time:', time()-start_time


class Download_many:
    '''
    Download many chapters
    >>> d = Download_many(sites['mangareader'], ['/naruto/4', '/naruto/5', '/naruto/6'])
    >>> d.execute()
    '''
    def __init__(self, site, paths):
        self.site = site
        self.paths = paths

    def execute(self):
        def worker_func(path):
            d = Download(self.site, path)
            d.execute()
        tasks = self.paths
        q = Gevent_queue(tasks, worker_func, workers=4)
        start_time = time()
        q.execute()
        print '>>> Overall time', time()-start_time


class Gevent_test:
    '''
    For testing if Gevent_queue is refactored correctly
    >>> q = Gevent_test()
    >>> q.execute()
    '''
    def execute(self):
        def worker(task):
            gevent.sleep(1)
            return True

        tasks = range(10)
        q = Gevent_queue(tasks, worker)
        q.execute()


if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('manga_name', help='Manga name')
    parser.add_argument('from_chapter', help='From chapter', type=int)
    parser.add_argument('to_chapter', help='To chapter', type=int)
    args = parser.parse_args()

    chapters = ['/' + args.manga_name + '/{0}'.format(x) for x in xrange(args.from_chapter, args.to_chapter+1)]
    d = Download_many(sites['mangareader'], chapters)
    d.execute()

from bs4 import BeautifulSoup
import gevent
from gevent.queue import Queue
from gevent import monkey
import requests
from time import time
from pprint import pprint
from PIL import Image
from StringIO import StringIO

monkey.patch_socket()


sites = {
    'mangareader': {
        'url': 'http://www.mangareader.net',
        'page_list': lambda p: ['http://www.mangareader.net' + x['value'] for x in p.find_all('option')],
        'img': lambda p: p.find(id='img').get('src')
    }
}


class Gevent_queue:
    def __init__(self, tasks, worker_func, workers=4):
        self.queue = Queue()
        self.tasks = tasks  # format is [(pagenum, pageurl)]
        self.workers = workers  # num of workers
        self.out = []  # list of results
        self.worker_func = worker_func

    def worker(self, n):
        while not self.queue.empty():
            start_time = time()
            task = self.queue.get()

            # execute worker function
            worker_out = self.worker_func(task)
            self.out.append(worker_out)

            print 'Worker {0} finished {1} in {2}'.format(n, task, time()-start_time)

    def execute(self):
        for task in self.tasks:
            self.queue.put_nowait(task)
        gevent.joinall([gevent.spawn(self.worker, x) for x in xrange(self.workers)])
        self.out.sort()
        return self.out


class Download:
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
            jpg = Image.open(StringIO(jpg_raw.content))

            # finish task
            return (task[0], img, jpg)

        # get page 1 + page 1 img + links to other pages
        print 'Getting page list'
        page_raw = requests.get(self.site['url'] + self.path)
        page = BeautifulSoup(page_raw.text, 'html.parser')
        img1 = self.site['img'](page)

        jpg_raw = requests.get(img1)
        jpg = Image.open(StringIO(jpg_raw.content))
        jpg.save(self.filename + '-0.jpg')

        # TODO save into a cbz

        # get list of pages
        pages = self.site['page_list'](page)
        pages = [(i+1,p) for i, p in enumerate(pages[1:])]

        # get multiple pages
        start_time = time()
        q = Gevent_queue(pages, worker_func=worker_func, workers=10)
        q_out = q.execute()

        # save the images
        # TODO save into a cbz
        for img in q_out:
            img[2].save(self.filename + '-' + str(img[0]) + '.jpg')

        print '>>> Time:', time()-start_time


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
    # q = Gevent_test()
    # q.execute()

    d = Download(sites['mangareader'], '/naruto/5')
    d.execute()


